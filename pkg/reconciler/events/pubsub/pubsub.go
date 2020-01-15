/*
Copyright 2019 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pubsub

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "pubsubs.events.cloud.google.com"
)

// Reconciler is the controller implementation for the PubSub source.
type Reconciler struct {
	*reconciler.Base

	// pubsubLister for reading pubsubs.
	pubsubLister listers.PubSubLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister

	receiveAdapterName string
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PubSub resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the PubSub resource with this namespace/name
	original, err := r.pubsubLister.PubSubs(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The PubSub resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("PubSub in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	pubsub := original.DeepCopy()

	reconcileErr := r.reconcile(ctx, pubsub)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		pubsub.Status.ObservedGeneration = pubsub.Generation
	}

	if equality.Semantic.DeepEqual(original.Status, pubsub.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, pubsub); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update PubSub status", zap.Error(uErr))
		r.Recorder.Eventf(pubsub, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for PubSub %q: %v", pubsub.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(pubsub, corev1.EventTypeNormal, "Updated", "Updated PubSub %q", pubsub.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(pubsub, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, pubsub *v1alpha1.PubSub) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pubsub", pubsub)))

	pubsub.Status.InitializeConditions()

	if pubsub.DeletionTimestamp != nil {
		// No finalizer needed, the pullsubscription will be garbage collected.
		return nil
	}

	ps, err := r.reconcilePullSubscription(ctx, pubsub)
	if err != nil {
		pubsub.Status.MarkPullSubscriptionFailed("PullSubscriptionReconcileFailed", "Failed to reconcile PullSubscription: %s", err.Error())
		return err
	}
	pubsub.Status.PropagatePullSubscriptionStatus(ps.Status.GetTopLevelCondition())

	// Sink has been resolved from the underlying PullSubscription, set it here.
	sinkURI, err := apis.ParseURL(ps.Status.SinkURI)
	if err != nil {
		pubsub.Status.SinkURI = nil
		return err
	} else {
		pubsub.Status.SinkURI = sinkURI
	}
	return nil
}

func (r *Reconciler) reconcilePullSubscription(ctx context.Context, source *v1alpha1.PubSub) (*pubsubv1alpha1.PullSubscription, error) {
	ps, err := r.pullsubscriptionLister.PullSubscriptions(source.Namespace).Get(source.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return nil, fmt.Errorf("failed to get PullSubscription: %w", err)
		}
		newPS := resources.MakePullSubscription(source.Namespace, source.Name, &source.Spec.PubSubSpec, source, source.Spec.Topic, r.receiveAdapterName, "", resourceGroup)
		logging.FromContext(ctx).Desugar().Debug("Creating PullSubscription", zap.Any("ps", newPS))
		ps, err = r.RunClientSet.PubsubV1alpha1().PullSubscriptions(newPS.Namespace).Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Error(err))
			return nil, fmt.Errorf("failed to create PullSubscription: %w", err)
		}
	}
	return ps, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PubSub) (*v1alpha1.PubSub, error) {
	source, err := r.pubsubLister.PubSubs(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// Check if there is anything to update.
	if equality.Semantic.DeepEqual(source.Status, desired.Status) {
		return source, nil
	}
	becomesReady := desired.Status.IsReady() && !source.Status.IsReady()

	// Don't modify the informers copy.
	existing := source.DeepCopy()
	existing.Status = desired.Status
	src, err := r.RunClientSet.EventsV1alpha1().PubSubs(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Desugar().Info("PubSub became ready", zap.Any("after", duration))
		r.Recorder.Event(source, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("PubSub %q became ready", source.Name))
		if metricErr := r.StatsReporter.ReportReady("PubSub", source.Namespace, source.Name, duration); metricErr != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to record ready for PubSub", zap.Error(metricErr))
		}
	}

	return src, err
}
