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
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
	psresources "github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "cloudpubsubsources.events.cloud.google.com"
)

// Reconciler is the controller implementation for the CloudPubSubSource source.
type Reconciler struct {
	*reconciler.Base

	// pubsubLister for reading cloudpubsubsources.
	pubsubLister listers.CloudPubSubSourceLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister

	receiveAdapterName string
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

	// Get the CloudPubSubSource resource with this namespace/name
	original, err := r.pubsubLister.CloudPubSubSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The CloudPubSubSource resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("CloudPubSubSource in work queue no longer exists")
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

	} else if uErr := r.updateStatus(ctx, original, pubsub); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update CloudPubSubSource status", zap.Error(uErr))
		r.Recorder.Eventf(pubsub, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for CloudPubSubSource %q: %v", pubsub.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(pubsub, corev1.EventTypeNormal, "Updated", "Updated CloudPubSubSource %q", pubsub.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(pubsub, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, pubsub *v1alpha1.CloudPubSubSource) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pubsub", pubsub)))

	pubsub.Status.InitializeConditions()

	// If GCP ServiceAccount is provided, get the corresponding k8s ServiceAccount.
	// kServiceAccount will be nil if GCP ServiceAccount is not provided or there is no corresponding k8s ServiceAccount.
	var kServiceAccount *corev1.ServiceAccount
	if pubsub.Spec.ServiceAccount != nil {
		kServiceAccountName := psresources.GenerateServiceAccountName(pubsub.Spec.ServiceAccount)
		ksa, err := r.serviceAccountLister.ServiceAccounts(pubsub.Namespace).Get(kServiceAccountName)
		if err != nil {
			if !apierrs.IsNotFound(err) {
				logging.FromContext(ctx).Desugar().Error("Failed to get k8s service account", zap.Error(err))
				return err
			}
		} else {
			kServiceAccount = ksa
		}
	}

	if pubsub.DeletionTimestamp != nil {
		// If k8s ServiceAccount exists and it only has one ownerReference, remove the corresponding GCP ServiceAccount iam policy binding.
		// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
		if kServiceAccount != nil && len(kServiceAccount.OwnerReferences) == 1 {
			logging.FromContext(ctx).Desugar().Debug("Removing iam policy binding.")
			psresources.RemoveIamPolicyBinding(ctx, *pubsub.Spec.ServiceAccount, kServiceAccount)
		}
		// The pullsubscription will be garbage collected.
		return nil
	}

	// If GCP ServiceAccount is provided, configure workload identity.
	if pubsub.Spec.ServiceAccount != nil {
		gServiceAccount := *pubsub.Spec.ServiceAccount
		// Create corresponding k8s ServiceAccount if doesn't exist, and add ownerReference to it.
		if err := r.CreateServiceAccount(ctx, pubsub, kServiceAccount); err != nil {
			return err
		}
		// Add iam policy binding to GCP ServiceAccount.
		if err := psresources.AddIamPolicyBinding(ctx, gServiceAccount, kServiceAccount); err != nil {
			return err
		}
	}

	ps, err := r.reconcilePullSubscription(ctx, pubsub)
	if err != nil {
		pubsub.Status.MarkPullSubscriptionFailed("PullSubscriptionReconcileFailed", "Failed to reconcile PullSubscription: %s", err.Error())
		return err
	}
	pubsub.Status.PropagatePullSubscriptionStatus(&ps.Status)

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

func (r *Reconciler) reconcilePullSubscription(ctx context.Context, source *v1alpha1.CloudPubSubSource) (*pubsubv1alpha1.PullSubscription, error) {
	ps, err := r.pullsubscriptionLister.PullSubscriptions(source.Namespace).Get(source.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return nil, fmt.Errorf("failed to get PullSubscription: %w", err)
		}
		args := &resources.PullSubscriptionArgs{
			Namespace:   source.Namespace,
			Name:        source.Name,
			Spec:        &source.Spec.PubSubSpec,
			Owner:       source,
			Topic:       source.Spec.Topic,
			Mode:        pubsubv1alpha1.ModePushCompatible,
			Labels:      resources.GetLabels(r.receiveAdapterName, source.Name),
			Annotations: resources.GetAnnotations(source.Annotations, resourceGroup),
		}
		newPS := resources.MakePullSubscription(args)
		logging.FromContext(ctx).Desugar().Debug("Creating PullSubscription", zap.Any("ps", newPS))
		ps, err = r.RunClientSet.PubsubV1alpha1().PullSubscriptions(newPS.Namespace).Create(newPS)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create PullSubscription", zap.Error(err))
			return nil, fmt.Errorf("failed to create PullSubscription: %w", err)
		}
	}
	return ps, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, original *v1alpha1.CloudPubSubSource, desired *v1alpha1.CloudPubSubSource) error {
	existing := original.DeepCopy()
	return pkgreconciler.RetryUpdateConflicts(func(attempts int) (err error) {
		// The first iteration tries to use the informer's state, subsequent attempts fetch the latest state via API.
		if attempts > 0 {
			existing, err = r.RunClientSet.EventsV1alpha1().CloudPubSubSources(desired.Namespace).Get(desired.Name, metav1.GetOptions{})
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
		_, err = r.RunClientSet.EventsV1alpha1().CloudPubSubSources(desired.Namespace).UpdateStatus(existing)

		if err == nil && becomesReady {
			// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
			duration := time.Since(existing.ObjectMeta.CreationTimestamp.Time)
			logging.FromContext(ctx).Desugar().Info("CloudPubSubSource became ready", zap.Any("after", duration))
			r.Recorder.Event(existing, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("CloudPubSubSource %q became ready", existing.Name))
			if metricErr := r.StatsReporter.ReportReady("CloudPubSubSource", existing.Namespace, existing.Name, duration); metricErr != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to record ready for CloudPubSubSource", zap.Error(metricErr))
			}
		}

		return err
	})
}

func (r *Reconciler) CreateServiceAccount(ctx context.Context, pubsub *v1alpha1.CloudPubSubSource, kServiceAccount *corev1.ServiceAccount) error {
	// If kServiceAccount doesn't exist, create it first.
	if kServiceAccount == nil {
		expect := resources.MakeServiceAccount(pubsub.Namespace, pubsub.Spec.ServiceAccount)
		logging.FromContext(ctx).Desugar().Debug("Creating k8s service account", zap.Any("kServiceAccount", expect))
		ksa, err := r.KubeClientSet.CoreV1().ServiceAccounts(expect.Namespace).Create(expect)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create k8s service account", zap.Error(err))
			return fmt.Errorf("failed to create k8s service account: %w", err)
		}
		kServiceAccount = ksa
	}
	// Add ownerReference to K8s ServiceAccount.
	expectOwnerReference := *kmeta.NewControllerRef(pubsub)
	control := false
	expectOwnerReference.Controller = &control
	if !resources.OwnerReferenceExists(kServiceAccount, expectOwnerReference) {
		kServiceAccount.OwnerReferences = append(kServiceAccount.OwnerReferences, expectOwnerReference)
		if _, err := r.KubeClientSet.CoreV1().ServiceAccounts(kServiceAccount.Namespace).Update(kServiceAccount); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to update OwnerReferences", zap.Error(err))
			return fmt.Errorf("failed to update OwnerReferences: %w", err)
		}
	}
	return nil
}
