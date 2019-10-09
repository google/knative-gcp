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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/resources"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	// TODO Reconciler name is currently the same as PubSubBase reconciler.
	//  Move reconciler.pubsub.reconciler.go to some other package and rename its reconciler.

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

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the PubSub resource with this namespace/name
	original, err := r.pubsubLister.PubSubs(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The PubSub resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("storage '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	csr := original.DeepCopy()

	reconcileErr := r.reconcile(ctx, csr)

	if equality.Semantic.DeepEqual(original.Status, csr.Status) &&
		equality.Semantic.DeepEqual(original.ObjectMeta, csr.ObjectMeta) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := r.updateStatus(ctx, csr); err != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		r.Logger.Warn("Failed to update PubSub Source status", zap.Error(err))
		return err
	}

	if reconcileErr != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		return reconcileErr
	}

	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha1.PubSub) error {
	source.Status.ObservedGeneration = source.Generation
	source.Status.InitializeConditions()

	if source.DeletionTimestamp != nil {
		// No finalizer needed, the pullsubscription will be garbage collected.
		return nil
	}

	ps, err := r.reconcilePullSubscription(ctx, source)
	if err != nil {
		r.Logger.Infof("Failed to reconcile PubSub: %s", err)
		return err
	}
	source.Status.PropagatePullSubscriptionStatus(ps.Status.GetCondition(apis.ConditionReady))

	r.Logger.Infof("Using %q as a cluster internal sink", ps.Status.SinkURI)
	uri, err := apis.ParseURL(ps.Status.SinkURI)
	if err != nil {
		return err
	}
	source.Status.SinkURI = uri
	r.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", source, ps)
	return nil
}

func (r *Reconciler) reconcilePullSubscription(ctx context.Context, source *v1alpha1.PubSub) (*pubsubv1alpha1.PullSubscription, error) {
	ps, err := r.pullsubscriptionLister.PullSubscriptions(source.Namespace).Get(source.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			r.Logger.Infof("Failed to get PullSubscriptions: %v", err)
			return nil, fmt.Errorf("failed to get pullsubscriptions: %v", err)
		}
		newPS := resources.MakePullSubscription(source.Namespace, source.Name, &source.Spec.PubSubSpec, source, source.Spec.Topic, r.receiveAdapterName, resourceGroup)
		r.Logger.Infof("Creating pullsubscription %+v", newPS)
		ps, err = r.RunClientSet.PubsubV1alpha1().PullSubscriptions(newPS.Namespace).Create(newPS)
		if err != nil {
			r.Logger.Infof("Failed to create PullSubscription: %v", err)
			return nil, fmt.Errorf("failed to create pullsubscription: %v", err)
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
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("PubSub %q became ready after %v", source.Name, duration)

		if err := r.StatsReporter.ReportReady("PubSub", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Storage, %v", err)
		}
	}

	return src, err
}
