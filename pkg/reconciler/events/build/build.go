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

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"github.com/google/knative-gcp/pkg/apis/events"
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	cloudbuildsourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudbuildsource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "cloudbuildsources.events.cloud.google.com"

	createFailedReason           = "PullSubscriptionCreateFailed"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
	reconciledSuccessReason      = "CloudBuildSourceReconciled"
)

// Reconciler is the controller implementation for the CloudBuildSource source.
type Reconciler struct {
	*intevents.PubSubBase

	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// buildLister for reading cloudbuildsources.
	buildLister listers.CloudBuildSourceLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister
}

// Check that our Reconciler implements Interface.
var _ cloudbuildsourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, build *v1.CloudBuildSource) pkgreconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("build", build)))

	build.Status.InitializeConditions()
	build.Status.ObservedGeneration = build.Generation
	// If ServiceAccountName is provided, reconcile workload identity.
	if build.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, build.Spec.Project, build); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile CloudBuildSource workload identity: %s", err.Error())
		}
	}
	_, event := r.PubSubBase.ReconcilePullSubscription(ctx, build, events.CloudBuildTopic, resourceGroup)
	if event != nil {
		return event
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudBuildSource reconciled: "%s/%s"`, build.Namespace, build.Name)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, build *v1.CloudBuildSource) pkgreconciler.Event {
	// If k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if build.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, build.Spec.Project, build); err != nil {
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete CloudBuildSource workload identity: %s", err.Error())
		}
	}
	return nil
}
