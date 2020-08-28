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

package scheduler

import (
	"context"

	"go.uber.org/zap"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	cloudschedulersourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudschedulersource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	gscheduler "github.com/google/knative-gcp/pkg/gclient/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	resourceGroup = "cloudschedulersources.events.cloud.google.com"

	deleteJobFailed              = "JobDeleteFailed"
	deletePubSubFailed           = "PubSubDeleteFailed"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	reconciledPubSubFailedReason = "PubSubReconcileFailed"
	reconciledFailedReason       = "JobReconcileFailed"
	reconciledSuccessReason      = "CloudSchedulerSourceReconciled"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

// Reconciler is the controller implementation for Google Cloud Scheduler Jobs.
type Reconciler struct {
	*intevents.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// schedulerLister for reading schedulers.
	schedulerLister listers.CloudSchedulerSourceLister

	createClientFn gscheduler.CreateFn
}

// Check that our Reconciler implements Interface.
var _ cloudschedulersourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, scheduler *v1.CloudSchedulerSource) reconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("scheduler", scheduler)))

	scheduler.Status.InitializeConditions()
	scheduler.Status.ObservedGeneration = scheduler.Generation

	// If ServiceAccountName is provided, reconcile workload identity.
	if scheduler.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, scheduler.Spec.Project, scheduler); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile CloudSchedulerSource workload identity: %s", err.Error())
		}
	}

	topic := resources.GenerateTopicName(scheduler)
	_, _, err := r.PubSubBase.ReconcilePubSub(ctx, scheduler, topic, resourceGroup)
	if err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledPubSubFailedReason, "Reconcile PubSub failed with: %s", err.Error())
	}

	jobName := resources.GenerateJobName(scheduler)
	err = r.reconcileJob(ctx, scheduler, topic, jobName)
	if err != nil {
		scheduler.Status.MarkJobNotReady(reconciledFailedReason, "Failed to reconcile CloudSchedulerSource job: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledFailedReason, "Reconcile Job failed with: %s", err.Error())
	}
	scheduler.Status.MarkJobReady(jobName)
	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudSchedulerSource reconciled: "%s/%s"`, scheduler.Namespace, scheduler.Name)
}

func (r *Reconciler) reconcileJob(ctx context.Context, scheduler *v1.CloudSchedulerSource, topic, jobName string) error {
	if scheduler.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(scheduler.Spec.Project, metadataClient.NewDefaultMetadataClient())
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return err
		}
		// Set the projectID in the status.
		scheduler.Status.ProjectID = projectID
	}

	client, err := r.createClientFn(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudSchedulerSource client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Check if the job exists.
	_, err = client.GetJob(ctx, &schedulerpb.GetJobRequest{Name: jobName})
	if err != nil {
		if st, ok := gstatus.FromError(err); !ok {
			logging.FromContext(ctx).Desugar().Error("Failed from CloudSchedulerSource client while retrieving CloudSchedulerSource job", zap.String("jobName", jobName), zap.Error(err))
			return err
		} else if st.Code() == codes.NotFound {
			// Create the job as it does not exist. For creation, we need a parent, extract it from the jobName.
			parent := resources.ExtractParentName(jobName)
			// Add jobName as customAttribute.
			customAttributes := map[string]string{
				v1.CloudSchedulerSourceJobName: jobName,
			}
			_, err = client.CreateJob(ctx, &schedulerpb.CreateJobRequest{
				Parent: parent,
				Job: &schedulerpb.Job{
					Name: jobName,
					Target: &schedulerpb.Job_PubsubTarget{
						PubsubTarget: &schedulerpb.PubsubTarget{
							TopicName:  resources.GeneratePubSubTargetTopic(scheduler, topic),
							Data:       []byte(scheduler.Spec.Data),
							Attributes: customAttributes,
						},
					},
					Schedule: scheduler.Spec.Schedule,
				},
			})
			if err != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to create CloudSchedulerSource job", zap.String("jobName", jobName), zap.Error(err))
				return err
			}
		} else {
			logging.FromContext(ctx).Desugar().Error("Failed from CloudSchedulerSource client while retrieving CloudSchedulerSource job", zap.String("jobName", jobName), zap.Any("errorCode", st.Code()), zap.Error(err))
			return err
		}
	}
	return nil
}

// deleteJob looks at the status.JobName and if non-empty,
// hence indicating that we have created a job successfully
// in the Scheduler, remove it.
func (r *Reconciler) deleteJob(ctx context.Context, scheduler *v1.CloudSchedulerSource) error {
	if scheduler.Status.JobName == "" {
		return nil
	}

	client, err := r.createClientFn(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudSchedulerSource client", zap.Error(err))
		scheduler.Status.MarkJobUnknown(deleteJobFailed, "Failed to create CloudSchedulerSource client: %s", err.Error())
		return err
	}
	defer client.Close()

	err = client.DeleteJob(ctx, &schedulerpb.DeleteJobRequest{Name: scheduler.Status.JobName})
	if err == nil {
		logging.FromContext(ctx).Desugar().Debug("Deleted CloudSchedulerSource job", zap.String("jobName", scheduler.Status.JobName))
		return nil
	}
	if st, ok := gstatus.FromError(err); !ok {
		logging.FromContext(ctx).Desugar().Error("Failed from CloudSchedulerSource client while deleting CloudSchedulerSource job", zap.String("jobName", scheduler.Status.JobName), zap.Error(err))
		scheduler.Status.MarkJobUnknown(deleteJobFailed, "Failed from CloudSchedulerSource client while deleting CloudSchedulerSource job: %s", err.Error())
		return err
	} else if st.Code() != codes.NotFound {
		logging.FromContext(ctx).Desugar().Error("Failed to delete CloudSchedulerSource job", zap.String("jobName", scheduler.Status.JobName), zap.Error(err))
		scheduler.Status.MarkJobUnknown(deleteJobFailed, "Failed to delete CloudSchedulerSource job: %s", err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, scheduler *v1.CloudSchedulerSource) reconciler.Event {
	// If k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if scheduler.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, scheduler.Spec.Project, scheduler); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete CloudSchedulerSource workload identity: %s", err.Error())
		}
	}

	logging.FromContext(ctx).Desugar().Debug("Deleting CloudSchedulerSource job")
	if err := r.deleteJob(ctx, scheduler); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deleteJobFailed, "Failed to delete CloudSchedulerSource job: %s", err.Error())
	}

	if err := r.PubSubBase.DeletePubSub(ctx, scheduler); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deletePubSubFailed, "Failed to delete CloudSchedulerSource PubSub: %s", err.Error())
	}

	return nil
}
