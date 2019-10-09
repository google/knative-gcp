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

package storage

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
	operations "github.com/google/knative-gcp/pkg/operations/storage"
	"github.com/google/knative-gcp/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Storage"

	finalizerName = controllerAgentName

	resourceGroup = "storages.events.cloud.google.com"
)

// Reconciler is the controller implementation for Google Cloud Storage (GCS) event
// notifications.
type Reconciler struct {
	*reconciler.PubSubBase

	// Image to use for launching jobs that operate on notifications
	NotificationOpsImage string

	// gcssourceclientset is a clientset for our own API group
	storageLister listers.StorageLister

	// For reading with jobs
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

	// Get the Storage resource with this namespace/name
	original, err := c.storageLister.Storages(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The Storage resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("storage '%s' in work queue no longer exists", key))
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
		c.Logger.Warn("Failed to update Storage Source status", zap.Error(err))
		return err
	}

	if reconcileErr != nil {
		// TODO: record the event (c.Recorder.Eventf(...
		return reconcileErr
	}

	return nil
}

func (c *Reconciler) reconcile(ctx context.Context, csr *v1alpha1.Storage) error {
	csr.Status.ObservedGeneration = csr.Generation
	// If notification / topic has been already configured, stash them here
	// since right below we remove them.
	notificationID := csr.Status.NotificationID
	topic := csr.Status.TopicID

	csr.Status.InitializeConditions()
	// And restore them.
	csr.Status.NotificationID = notificationID

	if topic == "" {
		topic = fmt.Sprintf("storage-%s", string(csr.UID))
	}

	// See if the source has been deleted.
	deletionTimestamp := csr.DeletionTimestamp

	if deletionTimestamp != nil {
		err := c.deleteNotification(ctx, csr)
		if err != nil {
			c.Logger.Infof("Unable to delete the Notification: %s", err)
			return err
		}
		err = c.PubSubBase.DeletePubSub(ctx, csr.Namespace, csr.Name)
		if err != nil {
			c.Logger.Infof("Unable to delete pubsub resources : %s", err)
			return fmt.Errorf("failed to delete pubsub resources: %s", err)
		}
		c.removeFinalizer(csr)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	err := c.ensureFinalizer(csr)
	if err != nil {
		return err
	}

	t, ps, err := c.PubSubBase.ReconcilePubSub(ctx, csr, topic, resourceGroup)
	if err != nil {
		c.Logger.Infof("Failed to reconcile PubSub: %s", err)
		return err
	}

	c.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	notification, err := c.reconcileNotification(ctx, csr)
	if err != nil {
		// TODO: Update status with this...
		c.Logger.Infof("Failed to reconcile Storage Notification: %s", err)
		csr.Status.MarkNotificationNotReady("NotificationNotReady", "Failed to create Storage notification: %s", err)
		return err
	}

	csr.Status.MarkNotificationReady()

	c.Logger.Infof("Reconciled Storage notification: %+v", notification)
	csr.Status.NotificationID = notification
	return nil
}

func (c *Reconciler) EnsureNotification(ctx context.Context, storage *v1alpha1.Storage) (ops.OpsJobStatus, error) {
	return c.ensureNotificationJob(ctx, operations.NotificationArgs{
		UID:        string(storage.UID),
		Image:      c.NotificationOpsImage,
		Action:     ops.ActionCreate,
		ProjectID:  storage.Status.ProjectID,
		Bucket:     storage.Spec.Bucket,
		TopicID:    storage.Status.TopicID,
		EventTypes: storage.Spec.EventTypes,
		Secret:     *storage.Spec.Secret,
		Owner:      storage,
	})
}

func (c *Reconciler) reconcileNotification(ctx context.Context, storage *v1alpha1.Storage) (string, error) {
	state, err := c.EnsureNotification(ctx, storage)
	if err != nil {
		c.Logger.Infof("EnsureNotification failed: %s", err)
	}

	if state == ops.OpsJobCreateFailed || state == ops.OpsJobCompleteFailed {
		return "", fmt.Errorf("Job %q failed to create or job failed", storage.Name)
	}

	if state != ops.OpsJobCompleteSuccessful {
		return "", fmt.Errorf("Job %q has not completed yet", storage.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, c.KubeClientSet, storage.Namespace, string(storage.UID), "create")
	if err != nil {
		return "", err
	}

	var result operations.NotificationActionResult
	if err := ops.GetOperationsResult(ctx, pod, &result); err != nil {
		return "", err
	}
	if result.Result {
		return result.NotificationId, nil
	}
	return "", fmt.Errorf("operation failed: %s", result.Error)
}

func (c *Reconciler) EnsureNotificationDeleted(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, bucket, notificationId string) (ops.OpsJobStatus, error) {
	return c.ensureNotificationJob(ctx, operations.NotificationArgs{
		UID:            UID,
		Image:          c.NotificationOpsImage,
		Action:         ops.ActionDelete,
		ProjectID:      project,
		Bucket:         bucket,
		NotificationId: notificationId,
		Secret:         secret,
		Owner:          owner,
	})
}

// deleteNotification looks at the status.NotificationID and if non-empty
// hence indicating that we have created a notification successfully
// in the Storage, remove it.
func (c *Reconciler) deleteNotification(ctx context.Context, storage *v1alpha1.Storage) error {
	if storage.Status.NotificationID == "" {
		return nil
	}

	state, err := c.EnsureNotificationDeleted(ctx, string(storage.UID), storage, *storage.Spec.Secret, storage.Spec.Project, storage.Spec.Bucket, storage.Status.NotificationID)

	if state != ops.OpsJobCompleteSuccessful {
		return fmt.Errorf("Job %q has not completed yet", storage.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, c.KubeClientSet, storage.Namespace, string(storage.UID), "delete")
	if err != nil {
		return err
	}
	// Check to see if the operation worked.
	var result operations.NotificationActionResult
	if err := ops.GetOperationsResult(ctx, pod, &result); err != nil {
		return err
	}
	if !result.Result {
		return fmt.Errorf("operation failed: %s", result.Error)
	}

	c.Logger.Infof("Deleted Notification: %q", storage.Status.NotificationID)
	storage.Status.NotificationID = ""
	return nil
}

func (c *Reconciler) ensureFinalizer(csr *v1alpha1.Storage) error {
	finalizers := sets.NewString(csr.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(csr.Finalizers, finalizerName),
			"resourceVersion": csr.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().Storages(csr.Namespace).Patch(csr.Name, types.MergePatchType, patch)
	return err

}

func (c *Reconciler) removeFinalizer(csr *v1alpha1.Storage) error {
	// Only remove our finalizer if it's the first one.
	if len(csr.Finalizers) == 0 || csr.Finalizers[0] != finalizerName {
		return nil
	}

	// For parity with merge patch for adding, also use patch for removing
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      csr.Finalizers[1:],
			"resourceVersion": csr.ResourceVersion,
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}
	_, err = c.RunClientSet.EventsV1alpha1().Storages(csr.Namespace).Patch(csr.Name, types.MergePatchType, patch)
	return err
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Storage) (*v1alpha1.Storage, error) {
	source, err := c.storageLister.Storages(desired.Namespace).Get(desired.Name)
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
	src, err := c.RunClientSet.EventsV1alpha1().Storages(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		c.Logger.Infof("Storage %q became ready after %v", source.Name, duration)

		if err := c.StatsReporter.ReportReady("Storage", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Storage, %v", err)
		}
	}

	return src, err
}

func (c *Reconciler) ensureNotificationJob(ctx context.Context, args operations.NotificationArgs) (ops.OpsJobStatus, error) {
	jobName := operations.NotificationJobName(args.Owner, args.Action)
	job, err := c.jobLister.Jobs(args.Owner.GetObjectMeta().GetNamespace()).Get(jobName)

	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		c.Logger.Debugw("Job not found, creating with:", zap.Any("args", args))

		args.Image = c.NotificationOpsImage

		job, err = operations.NewNotificationOps(args)
		if err != nil {
			return ops.OpsJobCreateFailed, err
		}

		job, err := c.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			c.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, err
		}

		c.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		c.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
		// TODO: Handle this case
		//	} else if !metav1.IsControlledBy(job, args.Owner) {
		//		return ops.OpsJobCreateFailed, fmt.Errorf("storage does not own job %q", jobName)
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
