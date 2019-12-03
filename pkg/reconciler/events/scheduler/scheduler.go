/*
Copyright 2017 The Kubernetes Authors.

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
	"cloud.google.com/go/compute/metadata"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc/codes"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	gscheduler "cloud.google.com/go/scheduler/apiv1"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	"github.com/google/knative-gcp/pkg/operations/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
	gstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Scheduler"

	finalizerName = controllerAgentName

	resourceGroup = "schedulers.events.cloud.google.com"
)

// Reconciler is the controller implementation for Google Cloud Scheduler Jobs.
type Reconciler struct {
	*pubsub.PubSubBase

	// Image to use for launching jobs that operate on Scheduler resources.
	SchedulerOpsImage string

	// gcssourceclientset is a clientset for our own API group
	schedulerLister listers.SchedulerLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Scheduler resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the Scheduler resource with this namespace/name
	original, err := r.schedulerLister.Schedulers(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The Scheduler resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("PubSub in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	scheduler := original.DeepCopy()

	reconcileErr := r.reconcile(ctx, scheduler)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		scheduler.Status.ObservedGeneration = scheduler.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, scheduler.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := r.updateFinalizers(ctx, scheduler); fErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Scheduler finalizers", zap.Error(fErr))
		r.Recorder.Eventf(scheduler, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Scheduler %q: %v", scheduler.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(scheduler, corev1.EventTypeNormal, "Updated", "Updated Scheduler %q finalizers", scheduler.Name)
	}

	if equality.Semantic.DeepEqual(original.Status, scheduler.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, scheduler); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Scheduler status", zap.Error(uErr))
		r.Recorder.Eventf(scheduler, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Storage %q: %v", scheduler.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(scheduler, corev1.EventTypeNormal, "Updated", "Updated Scheduler %q", scheduler.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(scheduler, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, scheduler *v1alpha1.Scheduler) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("scheduler", scheduler)))

	// If jobName / topic has been already configured, stash them here
	// since right below we remove them.
	jobName := scheduler.Status.JobName
	topic := scheduler.Status.TopicID

	scheduler.Status.InitializeConditions()

	// And restore them.
	scheduler.Status.JobName = jobName
	if topic == "" {
		topic = fmt.Sprintf("scheduler-%s", string(scheduler.UID))
	}

	// See if the source has been deleted.
	if scheduler.DeletionTimestamp != nil {
		logging.FromContext(ctx).Desugar().Debug("Deleting Scheduler job")
		if err := r.deleteSchedulerJob(ctx, scheduler); err != nil {
			scheduler.Status.MarkJobNotReady("JobDeleteFailed", "Failed to delete Scheduler job: %s", err.Error())
			return err
		}
		scheduler.Status.MarkJobNotReady("JobDeleted", "Successfully deleted Scheduler job: %s", scheduler.Status.JobName)
		scheduler.Status.JobName = ""

		if err := r.PubSubBase.DeletePubSub(ctx, scheduler.Namespace, scheduler.Name); err != nil {
			return err
		}
		removeFinalizer(scheduler)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	addFinalizer(scheduler)

	_, _, err := r.PubSubBase.ReconcilePubSub(ctx, scheduler, topic, resourceGroup)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to reconcile PubSub", zap.Error(err))
		return err
	}

	jobName, err = r.reconcileJob(ctx, scheduler, topic, resources.GenerateJobName(scheduler))
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to reconcile Scheduler job", zap.Error(err))
		scheduler.Status.MarkJobNotReady("JobReconcileFailed", "Failed to reconcile Scheduler job: %s", err.Error())
		return err
	}
	scheduler.Status.MarkJobReady(jobName)
	return nil
}

func (r *Reconciler) reconcileJob(ctx context.Context, scheduler *v1alpha1.Scheduler, topic, jobName string) (string, error) {
	if scheduler.Spec.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return "", err
		}
		scheduler.Spec.Project = project
	}

	client, err := gscheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create Scheduler client", zap.Error(err))
		return "", err
	}

	// Check if the job exists.
	job, err := client.GetJob(ctx, &schedulerpb.GetJobRequest{Name: jobName})
	if err != nil {
		if st, ok := gstatus.FromError(err); !ok {
			logging.FromContext(ctx).Desugar().Error("Failed from Scheduler client while retrieving Scheduler job", zap.String("jobName", jobName), zap.Error(err))
			return "", err
		} else if st.Code() == codes.NotFound {
			// Create the job as it does not exist.
			// For create we need a Parent, which from the jobName projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID,
			// is: projects/PROJECT_ID/locations/LOCATION_ID
			parent := jobName[0:strings.LastIndex(jobName, "/jobs/")]
			job, err = client.CreateJob(ctx, &schedulerpb.CreateJobRequest{
				Parent: parent,
				Job: &schedulerpb.Job{
					Name: jobName,
					Target: &schedulerpb.Job_PubsubTarget{
						PubsubTarget: &schedulerpb.PubsubTarget{
							TopicName: fmt.Sprintf("projects/%s/topics/%s", scheduler.Spec.Project, topic),
							Data:      []byte(scheduler.Spec.Data),
						},
					},
					Schedule: scheduler.Spec.Schedule,
				},
			})
			if err != nil {
				logging.FromContext(ctx).Desugar().Error("Failed to create Scheduler job", zap.String("jobName", jobName), zap.Error(err))
				return "", err
			}
		} else {
			logging.FromContext(ctx).Desugar().Error("Failed from Scheduler client while retrieving Scheduler job", zap.String("jobName", jobName), zap.Any("errorCode", st.Code()), zap.Error(err))
			return "", err
		}
	}
	return job.Name, nil
}

// deleteSchedulerJob looks at the status.JobName and if non-empty
// hence indicating that we have created a Job successfully
// in the Scheduler, remove it.
func (r *Reconciler) deleteSchedulerJob(ctx context.Context, scheduler *v1alpha1.Scheduler) error {
	if scheduler.Status.JobName == "" {
		return nil
	}

	state, err := r.EnsureSchedulerJobDeleted(ctx, string(scheduler.UID), scheduler, *scheduler.Spec.Secret, scheduler.Status.JobName)

	if state != ops.OpsJobCompleteSuccessful {
		return fmt.Errorf("Job %q has not completed yet", scheduler.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, r.KubeClientSet, scheduler.Namespace, string(scheduler.UID), "delete")
	if err != nil {
		return err
	}

	// Check to see if the operation worked.
	var result operations.JobActionResult
	if err := ops.GetOperationsResult(ctx, pod, &result); err != nil {
		return err
	}
	if !result.Result {
		return fmt.Errorf("operation failed: %s", result.Error)
	}

	r.Logger.Infof("Deleted Notification: %q", scheduler.Status.JobName)
	scheduler.Status.JobName = ""
	return nil
}

func addFinalizer(s *v1alpha1.Scheduler) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.Scheduler) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Scheduler) (*v1alpha1.Scheduler, error) {
	source, err := r.schedulerLister.Schedulers(desired.Namespace).Get(desired.Name)
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
	src, err := r.RunClientSet.EventsV1alpha1().Schedulers(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Desugar().Info("Scheduler became ready after", zap.Any("duration", duration))

		if err := r.StatsReporter.ReportReady("Scheduler", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Desugar().Info("Railed to record ready for Scheduler", zap.Error(err))
		}
	}
	return src, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Scheduler) (*v1alpha1.Scheduler, bool, error) {
	scheduler, err := r.schedulerLister.Schedulers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := scheduler.DeepCopy()

	var finalizers []string

	// If there's nothing to update, just return.
	existingFinalizers := sets.NewString(existing.Finalizers...)
	desiredFinalizers := sets.NewString(desired.Finalizers...)

	if desiredFinalizers.Has(finalizerName) {
		if existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Add the finalizer.
		finalizers = append(existing.Finalizers, finalizerName)
	} else {
		if !existingFinalizers.Has(finalizerName) {
			// Nothing to do.
			return desired, false, nil
		}
		// Remove the finalizer.
		existingFinalizers.Delete(finalizerName)
		finalizers = existingFinalizers.List()
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      finalizers,
			"resourceVersion": existing.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return desired, false, err
	}

	update, err := r.RunClientSet.EventsV1alpha1().Schedulers(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}
