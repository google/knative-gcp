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
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	cloudpubsubsourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1alpha1/cloudpubsubsource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
)

const (
	resourceGroup = "cloudpubsubsources.events.cloud.google.com"

	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	reconciledSuccessReason      = "CloudPubSubSourceReconciled"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

// Reconciler is the controller implementation for the CloudPubSubSource source.
type Reconciler struct {
	*pubsub.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// pubsubLister for reading cloudpubsubsources.
	pubsubLister listers.CloudPubSubSourceLister
	// pullsubscriptionLister for reading pullsubscriptions.
	pullsubscriptionLister pubsublisters.PullSubscriptionLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister
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

	_, event := r.PubSubBase.ReconcilePullSubscription(ctx, pubsub, pubsub.Spec.Topic, resourceGroup, true)
	if event != nil {
		return event
	}
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudPubSubSource reconciled: "%s/%s"`, pubsub.Namespace, pubsub.Name)
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
