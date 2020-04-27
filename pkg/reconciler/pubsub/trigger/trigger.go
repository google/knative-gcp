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

package trigger

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	tracingconfig "knative.dev/pkg/tracing/config"

	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/pubsub/v1alpha1/trigger"
	listers "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	gtrigger "github.com/google/knative-gcp/pkg/gclient/trigger"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	resourceGroup = "triggers.pubsub.cloud.google.com"

	deleteTriggerFailed              = "TriggerDeleteFailed"
	deleteWorkloadIdentityFailed     = "WorkloadIdentityDeleteFailed"
	reconciledEventFlowTriggerFailed = "TriggerReconcileFailed"
	reconciledTriggerFailed          = "TriggerReconcileFailed"
	reconciledSuccess                = "TriggerReconciled"
	workloadIdentityFailed           = "WorkloadIdentityReconcileFailed"
)

// Reconciler is the controller implementation for Google Cloud Google (GCS) event
// triggers.
type Reconciler struct {
	*pubsub.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity

	// triggerLister for reading triggers.
	triggerLister listers.TriggerLister

	// serviceLister index properties about services.
	serviceLister servinglisters.ServiceLister
	// serviceAccountLister for reading serviceAccounts.
	serviceAccountLister corev1listers.ServiceAccountLister

	publisherImage string
	tracingConfig  *tracingconfig.Config

	// createClientFn is the function used to create the Trigger client that interacts with EventFlow.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gtrigger.CreateFn
}

// Check that our Reconciler implements Interface.
var _ triggerreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, trigger *v1alpha1.Trigger) reconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("trigger", trigger)))

	trigger.Status.InitializeConditions()
	trigger.Status.ObservedGeneration = trigger.Generation

	// If GCP ServiceAccount is provided, reconcile workload identity.
	if trigger.Spec.GoogleServiceAccount != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, trigger.Spec.Project, trigger); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile Trigger workload identity: %s", err.Error())
		}
	}

	eventflow_trigger, err := r.reconcileTrigger(ctx, trigger)
	if err != nil {
		trigger.Status.MarkTriggerNotReady(reconciledEventFlowTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledEventFlowTriggerFailed, "Failed to reconcile Trigger EventFlow trigger: %s", err.Error())
	}
	trigger.Status.MarkTriggerReady(eventflow_trigger)

	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccess, `Trigger reconciled: "%s/%s"`, trigger.Namespace, trigger.Name)
}

func (r *Reconciler) reconcileTrigger(ctx context.Context, trigger *v1alpha1.Trigger) (string, error) {
	if trigger.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(trigger.Spec.Project)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return "", err
		}
		// Set the projectID in the status.
		trigger.Status.ProjectID = projectID
	}

	// create the triggers client
	client, err := r.createClientFn(ctx, trigger.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Event Flow client", zap.Error(err))
		return "", err
	}
	defer client.Close()

	t := client.Trigger(trigger.Spec.Trigger)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Event Flow trigger exists", zap.Error(err))
		return "", err
	}

	if !exists {
		t, err = client.CreateTrigger(ctx, trigger.Spec.Trigger, trigger.Spec.SourceType, trigger.Spec.Filters)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to create trigger", zap.Error(err))
			return "", err
		}
	}
	return t.ID(), nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, trigger *v1alpha1.Trigger) reconciler.Event {
	if trigger.Status.TriggerID == "" {
		return nil
	}
	// If k8s ServiceAccount exists and it only has one ownerReference, remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if trigger.Spec.GoogleServiceAccount != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, trigger.Spec.Project, trigger); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete Trigger workload identity: %s", err.Error())
		}
	}

	// At this point the project ID should have been populated in the status.
	// Querying EventFlow as the trigger could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx, trigger.Status.ProjectID)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Event Flow client", zap.Error(err))
		return err
	}
	defer client.Close()
	t := client.Trigger(trigger.Status.TriggerID)
	exists, err := t.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to verify Event Flow trigger exists", zap.Error(err))
		return err
	}
	if exists {
		// Delete the trigger.
		if err := t.Delete(ctx); err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to delete Event Flow trigger", zap.Error(err))
			return err
		}
	}

	// ok to remove finalizer.
	return nil
}
