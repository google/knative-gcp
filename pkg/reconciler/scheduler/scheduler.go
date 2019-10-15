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
	operations "github.com/google/knative-gcp/pkg/operations/scheduler"
	"github.com/google/knative-gcp/pkg/reconciler"
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
	*reconciler.PubSubBase

	// Image to use for launching jobs that operate on Scheduler resources.
	SchedulerOpsImage string

	// gcssourceclientset is a clientset for our own API group
	schedulerLister listers.SchedulerLister

	// For readling with jobs
	jobLister batchv1listers.JobLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Scheduler resource with this namespace/name
	original, err := c.schedulerLister.Schedulers(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The Scheduler resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("scheduler '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	csr := original.DeepCopy()

	reconcileErr := c.reconcile(ctx, csr)

	if equality.Semantic.DeepEqual(original.Status, csr.Status) &&
		equality.Semantic.DeepEqual(original.ObjectMeta, csr.ObjectMeta) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(ctx, csr); err != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		c.Logger.Warn("Failed to update Scheduler status", zap.Error(err))
		return err
	}

	if reconcileErr != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		return reconcileErr
	}

	return nil
}

func (c *Reconciler) reconcile(ctx context.Context, s *v1alpha1.Scheduler) error {
	s.Status.InitializeConditions()

	topic := fmt.Sprintf("scheduler-%s", string(s.UID))

	// See if the Scheduler has been deleted.
	deletionTimestamp := s.DeletionTimestamp

	if deletionTimestamp != nil {
		err := c.deleteSchedulerJob(ctx, s)
		if err != nil {
			c.Logger.Infof("Unable to delete the scheduler job: %s", err)
			return err
		}
		err = c.PubSubBase.DeletePubSub(ctx, s.Namespace, s.Name)
		if err != nil {
			c.Logger.Infof("Unable to delete pubsub resources : %s", err)
			return fmt.Errorf("failed to delete pubsub resources: %s", err)
		}
		c.removeFinalizer(s)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	err := c.ensureFinalizer(s)
	if err != nil {
		return err
	}

	t, ps, err := c.PubSubBase.ReconcilePubSub(ctx, s, topic, resourceGroup)
	if err != nil {
		c.Logger.Infof("Failed to reconcile PubSub: %s", err)
		return err
	}

	c.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	jobName := fmt.Sprintf("projects/%s/locations/%s/jobs/cre-scheduler-%s", t.Status.ProjectID, s.Spec.Location, string(s.UID))

	retJobName, err := c.reconcileNotification(ctx, s, topic, jobName)
	if err != nil {
		// TODO: Update status with this...
		c.Logger.Infof("Failed to reconcile Scheduler Job: %s", err)
		s.Status.MarkJobNotReady("JobNotReady", "Failed to create Scheduler Job: %s", err)
		return err
	}

	s.Status.MarkJobReady(retJobName)
	c.Logger.Infof("Reconciled Scheduler notification: %q", retJobName)
	return nil
}

func (c *Reconciler) EnsureSchedulerJob(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, topic, jobName, schedule, data string) (ops.OpsJobStatus, error) {
	return c.ensureSchedulerJob(ctx, operations.JobArgs{
		UID:      UID,
		Image:    c.SchedulerOpsImage,
		Action:   ops.ActionCreate,
		TopicID:  topic,
		JobName:  jobName,
		Secret:   secret,
		Owner:    owner,
		Schedule: schedule,
		Data:     data,
	})
}

func (c *Reconciler) reconcileNotification(ctx context.Context, scheduler *v1alpha1.Scheduler, topic, jobName string) (string, error) {
	state, err := c.EnsureSchedulerJob(ctx, string(scheduler.UID), scheduler, *scheduler.Spec.Secret, topic, jobName, scheduler.Spec.Schedule, scheduler.Spec.Data)

	if state == ops.OpsJobCreateFailed || state == ops.OpsJobCompleteFailed {
		return "", fmt.Errorf("Job %q failed to create or job failed", scheduler.Name)
	}

	if state != ops.OpsJobCompleteSuccessful {
		return "", fmt.Errorf("Job %q has not completed yet", scheduler.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, c.KubeClientSet, scheduler.Namespace, string(scheduler.UID), "create")
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

func (c *Reconciler) EnsureSchedulerJobDeleted(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, jobName string) (ops.OpsJobStatus, error) {
	return c.ensureSchedulerJob(ctx, operations.JobArgs{
		UID:     UID,
		Image:   c.SchedulerOpsImage,
		Action:  ops.ActionDelete,
		JobName: jobName,
		Secret:  secret,
		Owner:   owner,
	})
}

// deleteSchedulerJob looks at the status.NotificationID and if non-empty
// hence indicating that we have created a notification successfully
// in the Scheduler, remove it.
func (c *Reconciler) deleteSchedulerJob(ctx context.Context, scheduler *v1alpha1.Scheduler) error {
	if scheduler.Status.JobName == "" {
		return nil
	}

	state, err := c.EnsureSchedulerJobDeleted(ctx, string(scheduler.UID), scheduler, *scheduler.Spec.Secret, scheduler.Status.JobName)

	if state != ops.OpsJobCompleteSuccessful {
		return fmt.Errorf("Job %q has not completed yet", scheduler.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, c.KubeClientSet, scheduler.Namespace, string(scheduler.UID), "delete")
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

	c.Logger.Infof("Deleted Notification: %q", scheduler.Status.JobName)
	scheduler.Status.JobName = ""
	return nil
}

func (c *Reconciler) ensureFinalizer(s *v1alpha1.Scheduler) error {
	finalizers := sets.NewString(s.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(s.Finalizers, finalizerName),
			"resourceVersion": s.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().Schedulers(s.Namespace).Patch(s.Name, types.MergePatchType, patch)
	return err

}

func (c *Reconciler) removeFinalizer(s *v1alpha1.Scheduler) error {
	// Only remove our finalizer if it's the first one.
	if len(s.Finalizers) == 0 || s.Finalizers[0] != finalizerName {
		return nil
	}

	// For parity with merge patch for adding, also use patch for removing
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      s.Finalizers[1:],
			"resourceVersion": s.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().Schedulers(s.Namespace).Patch(s.Name, types.MergePatchType, patch)
	return err
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Scheduler) (*v1alpha1.Scheduler, error) {
	source, err := c.schedulerLister.Schedulers(desired.Namespace).Get(desired.Name)
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
	src, err := c.RunClientSet.EventsV1alpha1().Schedulers(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Scheduler %q became ready after %v", source.Name, duration)

		if err := c.StatsReporter.ReportReady("Scheduler", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Scheduler, %v", err)
		}
	}

	return src, err
}

func (c *Reconciler) ensureSchedulerJob(ctx context.Context, args operations.JobArgs) (ops.OpsJobStatus, error) {
	jobName := operations.SchedulerJobName(args.Owner, args.Action)
	job, err := c.jobLister.Jobs(args.Owner.GetObjectMeta().GetNamespace()).Get(jobName)

	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating with:", zap.Any("args", args))

		args.Image = c.SchedulerOpsImage

		job, err = operations.NewJobOps(args)
		if err != nil {
			return ops.OpsJobCreateFailed, err
		}

		job, err := c.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, nil
		}

		c.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
		// TODO: Handle this case
		//	} else if !metav1.IsControlledBy(job, args.Owner) {
		//		return ops.OpsJobCreateFailed, fmt.Errorf("scheduler does not own job %q", jobName)
	}

	if ops.IsJobComplete(job) {
		c.Logger.Debugw("Job is complete.")
		if ops.IsJobSucceeded(job) {
			return ops.OpsJobCompleteSuccessful, nil
		} else if ops.IsJobFailed(job) {
			return ops.OpsJobCompleteFailed, errors.New(ops.JobFailedMessage(job))
		}
	}
	c.Logger.Debug("Job still active.", zap.Any("job", job))
	return ops.OpsJobOngoing, nil
}
