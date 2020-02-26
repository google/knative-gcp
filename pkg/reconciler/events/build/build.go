/*
Copyright 2020 Google LLC

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

package build

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "cloudbuildsources.events.cloud.google.com"
)

// Reconciler is the controller implementation for the CloudBuildSource source.
type Reconciler struct {
	*pubsub.PubSubBase

	// buildLister for reading cloudbuildsources.
	buildLister listers.CloudBuildSourceLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CloudPubSubSource resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the CloudBuildSource resource with this namespace/name
	original, err := r.buildLister.CloudBuildSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The CloudBuildSource resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("CloudBuildSource in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	build := original.DeepCopy()

	reconcileErr := r.reconcile(ctx, build)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		build.Status.ObservedGeneration = build.Generation
	}

	if equality.Semantic.DeepEqual(original.Status, build.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if uErr := r.updateStatus(ctx, original, build); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update CloudBuildSource status", zap.Error(uErr))
		r.Recorder.Eventf(build, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for CloudBuildSource %q: %v", build.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(build, corev1.EventTypeNormal, "Updated", "Updated CloudBuildSource %q", build.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(build, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, build *v1alpha1.CloudBuildSource) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("build", build)))

	build.Status.InitializeConditions()

	if build.DeletionTimestamp != nil {
		// No finalizer needed, the pullsubscription will be garbage collected.
		return nil
	}

	_, err := r.PubSubBase.ReconcilePullSubscription(ctx, build, build.Spec.Topic, resourceGroup, false)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to reconcile PullSubscription", zap.Error(err))
		return err
	}
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, original *v1alpha1.CloudBuildSource, desired *v1alpha1.CloudBuildSource) error {
	existing := original.DeepCopy()
	return pkgreconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		// The first iteration tries to use the informer's state, subsequent attempts fetch the latest state via API.
		if attempts > 0 {
			existing, err = r.RunClientSet.EventsV1alpha1().CloudBuildSources(desired.Namespace).Get(desired.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		// Check if there is anything to update.
		if equality.Semantic.DeepEqual(existing.Status, desired.Status) {
			return nil
		}
		becomesReady := desired.Status.IsReady() && !existing.Status.IsReady()

		existing.Status = desired.Status
		_, err = r.RunClientSet.EventsV1alpha1().CloudBuildSources(desired.Namespace).UpdateStatus(existing)

		if err == nil && becomesReady {
			// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
			duration := time.Since(existing.ObjectMeta.CreationTimestamp.Time)
			logging.FromContext(ctx).Desugar().Info("CloudBuildSource became ready", zap.Any("after", duration))
			r.Recorder.Event(existing, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("CloudBuildSource %q became ready", existing.Name))
			if metricErr := r.StatsReporter.ReportReady("CloudBuildSource", existing.Namespace, existing.Name, duration); metricErr != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to record ready for CloudBuildSource", zap.Error(metricErr))
			}
		}

		return err
	})
}
