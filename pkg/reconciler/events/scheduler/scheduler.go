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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	"github.com/google/knative-gcp/pkg/operations/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
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
		logging.FromContext(ctx).Desugar().Debug("Deleting Storage notification")
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

	jobName := fmt.Sprintf("projects/%s/locations/%s/jobs/cre-scheduler-%s", scheduler.Status.ProjectID, scheduler.Spec.Location, string(scheduler.UID))

	retJobName, err := r.reconcileNotification(ctx, scheduler, topic, jobName)
	if err != nil {
		// TODO: Update status with this...
		r.Logger.Infof("Failed to reconcile Scheduler Job: %scheduler", err)
		scheduler.Status.MarkJobNotReady("JobNotReady", "Failed to create Scheduler Job: %scheduler", err)
		return err
	}

	scheduler.Status.MarkJobReady(retJobName)
	r.Logger.Infof("Reconciled Scheduler notification: %q", retJobName)
	return nil
}

func (r *Reconciler) EnsureSchedulerJob(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, topic, jobName, schedule, data string) (ops.OpsJobStatus, error) {
	return r.ensureSchedulerJob(ctx, operations.JobArgs{
		UID:      UID,
		Image:    r.SchedulerOpsImage,
		Action:   ops.ActionCreate,
		TopicID:  topic,
		JobName:  jobName,
		Secret:   secret,
		Owner:    owner,
		Schedule: schedule,
		Data:     data,
	})
}

func (r *Reconciler) reconcileNotification(ctx context.Context, scheduler *v1alpha1.Scheduler, topic, jobName string) (string, error) {
	state, err := r.EnsureSchedulerJob(ctx, string(scheduler.UID), scheduler, *scheduler.Spec.Secret, topic, jobName, scheduler.Spec.Schedule, scheduler.Spec.Data)

	if state == ops.OpsJobCreateFailed || state == ops.OpsJobCompleteFailed {
		return "", fmt.Errorf("Job %q failed to create or job failed", scheduler.Name)
	}

	if state != ops.OpsJobCompleteSuccessful {
		return "", fmt.Errorf("Job %q has not completed yet", scheduler.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, r.KubeClientSet, scheduler.Namespace, string(scheduler.UID), "create")
	if err != nil {
		return "", err
	}

	var result operations.JobActionResult
	if err := ops.GetOperationsResult(ctx, pod, &result); err != nil {
		return "", err
	}
	if result.Result {
		return result.JobName, nil
	}
	return "", fmt.Errorf("operation failed: %s", result.Error)
}

func (r *Reconciler) EnsureSchedulerJobDeleted(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, jobName string) (ops.OpsJobStatus, error) {
	return r.ensureSchedulerJob(ctx, operations.JobArgs{
		UID:     UID,
		Image:   r.SchedulerOpsImage,
		Action:  ops.ActionDelete,
		JobName: jobName,
		Secret:  secret,
		Owner:   owner,
	})
}

// deleteSchedulerJob looks at the status.NotificationID and if non-empty
// hence indicating that we have created a notification successfully
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

func (r *Reconciler) ensureSchedulerJob(ctx context.Context, args operations.JobArgs) (ops.OpsJobStatus, error) {
	jobName := operations.SchedulerJobName(args.Owner, args.Action)
	job, err := r.jobLister.Jobs(args.Owner.GetObjectMeta().GetNamespace()).Get(jobName)

	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		r.Logger.Debugw("Job not found, creating with:", zap.Any("args", args))

		args.Image = r.SchedulerOpsImage

		job, err = operations.NewJobOps(args)
		if err != nil {
			return ops.OpsJobCreateFailed, err
		}

		job, err := r.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			r.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, nil
		}

		r.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		r.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
		// TODO: Handle this case
		//	} else if !metav1.IsControlledBy(job, args.Owner) {
		//		return ops.OpsJobCreateFailed, fmt.Errorf("scheduler does not own job %q", jobName)
	}

	if ops.IsJobFailed(job) {
		r.Logger.Debugw("Job has failed.")
		return ops.OpsJobCompleteFailed, errors.New(ops.JobFailedMessage(job))
	}

	if ops.IsJobComplete(job) {
		r.Logger.Debugw("Job is complete.")
		return ops.OpsJobCompleteSuccessful, nil
	}

	r.Logger.Debug("Job still active.", zap.Any("job", job))
	return ops.OpsJobOngoing, nil
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
