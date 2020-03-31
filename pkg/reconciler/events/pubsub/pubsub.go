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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	cloudpubsubsourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudpubsubsource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub/resources"
)

const (
	resourceGroup = "cloudpubsubsources.events.cloud.google.com"

	createFailedReason           = "PullSubscriptionCreateFailed"
	getFailedReason              = "PullSubscriptionGetFailed"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	reconciledSuccessReason      = "CloudPubSubSourceReconciled"
	reconciledFailedReason       = "PullSubscriptionReconcileFailed"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

// Reconciler is the controller implementation for the CloudPubSubSource source.
type Reconciler struct {
	*reconciler.Base
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// pubsubLister for reading cloudpubsubsources.
	pubsubLister listers.CloudPubSubSourceLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister
	receiveAdapterName   string
}

// Check that our Reconciler implements Interface.
var _ cloudpubsubsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, pubsub *v1alpha1.CloudPubSubSource) pkgreconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("pubsub", pubsub)))

	pubsub.Status.InitializeConditions()
	pubsub.Status.ObservedGeneration = pubsub.Generation

	// If GCP ServiceAccount is provided, reconcile workload identity.
	if pubsub.Spec.GoogleServiceAccount != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, pubsub.Spec.Project, pubsub); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile CloudPubSubSource workload identity: %s", err.Error())
		}
	}

	ps, event := r.reconcilePullSubscription(ctx, pubsub)
	if event != nil {
		pubsub.Status.MarkPullSubscriptionFailed(reconciledFailedReason, "Failed to reconcile PullSubscription: %s", event.Error())
		return event
	}
	pubsub.Status.PropagatePullSubscriptionStatus(&ps.Status)
	pubsub.Status.SinkURI = ps.Status.SinkURI
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudPubSubSource reconciled: "%s/%s"`, pubsub.Namespace, pubsub.Name)
}

func (r *Reconciler) reconcilePullSubscription(ctx context.Context, source *v1alpha1.CloudPubSubSource) (*pubsubv1alpha1.PullSubscription, pkgreconciler.Event) {
	ps, err := r.pullsubscriptionLister.PullSubscriptions(source.Namespace).Get(source.Name)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Desugar().Error("Failed to get PullSubscription", zap.Error(err))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, getFailedReason, "Getting PullSubscription failed with: %s", err.Error())
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
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, createFailedReason, "Creating PullSubscription failed with: %s", err.Error())
		}
	}
	return ps, nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, pubsub *v1alpha1.CloudPubSubSource) pkgreconciler.Event {
	// If k8s ServiceAccount exists and it only has one ownerReference, remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if pubsub.Spec.GoogleServiceAccount != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, pubsub.Spec.Project, pubsub); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete CloudPubSubSource workload identity: %s", err.Error())
		}
	}
	return nil
}
