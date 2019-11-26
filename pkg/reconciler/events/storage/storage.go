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
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	ops "github.com/google/knative-gcp/pkg/operations"
	"github.com/google/knative-gcp/pkg/operations/storage"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// ReconcilerName is the name of the reconciler
	reconcilerName = "Storage"

	finalizerName = controllerAgentName

	resourceGroup = "storages.events.cloud.google.com"
)

// Reconciler is the controller implementation for Google Cloud Storage (GCS) event
// notifications.
type Reconciler struct {
	*pubsub.PubSubBase

	// Image to use for launching jobs that operate on notifications
	NotificationOpsImage string

	// storageLister is the Storage lister.
	storageLister listers.StorageLister
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Storage resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the Storage resource with this namespace/name
	original, err := r.storageLister.Storages(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The Storage resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("PubSub in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	storage := original.DeepCopy()

	reconcileErr := r.reconcile(ctx, storage)

	// If no error is returned, mark the observed generation.
	if reconcileErr == nil {
		storage.Status.ObservedGeneration = storage.Generation
	}

	if equality.Semantic.DeepEqual(original.Finalizers, storage.Finalizers) {
		// If we didn't change finalizers then don't call updateFinalizers.

	} else if _, updated, fErr := r.updateFinalizers(ctx, storage); fErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Storage finalizers", zap.Error(fErr))
		r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for Storage %q: %v", storage.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(storage, corev1.EventTypeNormal, "Updated", "Updated Storage %q finalizers", storage.Name)
	}

	if equality.Semantic.DeepEqual(original.Status, storage.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, storage); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update Storage status", zap.Error(uErr))
		r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Storage %q: %v", storage.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(storage, corev1.EventTypeNormal, "Updated", "Updated Storage %q", storage.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(storage, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, storage *v1alpha1.Storage) error {
	// If notification / topic has been already configured, stash them here
	// since right below we remove them.
	notificationID := storage.Status.NotificationID
	topic := storage.Status.TopicID

	storage.Status.InitializeConditions()
	// And restore them.
	storage.Status.NotificationID = notificationID

	if topic == "" {
		topic = fmt.Sprintf("storage-%s", string(storage.UID))
	}

	// See if the source has been deleted.
	deletionTimestamp := storage.DeletionTimestamp

	if deletionTimestamp != nil {
		err := r.deleteNotification(ctx, storage)
		if err != nil {
			r.Logger.Infof("Unable to delete the Notification: %s", err)
			return err
		}
		err = r.PubSubBase.DeletePubSub(ctx, storage.Namespace, storage.Name)
		if err != nil {
			r.Logger.Infof("Unable to delete pubsub resources : %s", err)
			return fmt.Errorf("failed to delete pubsub resources: %s", err)
		}
		removeFinalizer(storage)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	addFinalizer(storage)

	t, ps, err := r.PubSubBase.ReconcilePubSub(ctx, storage, topic, resourceGroup)
	if err != nil {
		r.Logger.Infof("Failed to reconcile PubSub: %s", err)
		return err
	}

	r.Logger.Infof("Reconciled: PubSub: %+v PullSubscription: %+v", t, ps)

	notification, err := r.reconcileNotification(ctx, storage)
	if err != nil {
		// TODO: Update status with this...
		r.Logger.Infof("Failed to reconcile Storage Notification: %s", err)
		storage.Status.MarkNotificationNotReady("NotificationNotReady", "Failed to create Storage notification: %s", err)
		return err
	}

	storage.Status.MarkNotificationReady()

	r.Logger.Infof("Reconciled Storage notification: %+v", notification)
	storage.Status.NotificationID = notification
	return nil
}

func (r *Reconciler) EnsureNotification(ctx context.Context, storage *v1alpha1.Storage) (ops.OpsJobStatus, error) {
	return r.ensureNotificationJob(ctx, operations.NotificationArgs{
		UID:        string(storage.UID),
		Image:      r.NotificationOpsImage,
		Action:     ops.ActionCreate,
		ProjectID:  storage.Status.ProjectID,
		Bucket:     storage.Spec.Bucket,
		TopicID:    storage.Status.TopicID,
		EventTypes: storage.Spec.EventTypes,
		Secret:     *storage.Spec.Secret,
		Owner:      storage,
	})
}

func (r *Reconciler) reconcileNotification(ctx context.Context, storage *v1alpha1.Storage) (string, error) {
	state, err := r.EnsureNotification(ctx, storage)
	if err != nil {
		r.Logger.Infof("EnsureNotification failed: %s", err)
	}

	if state == ops.OpsJobCreateFailed || state == ops.OpsJobCompleteFailed {
		return "", fmt.Errorf("Job %q failed to create or job failed", storage.Name)
	}

	if state != ops.OpsJobCompleteSuccessful {
		return "", fmt.Errorf("Job %q has not completed yet", storage.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, r.KubeClientSet, storage.Namespace, string(storage.UID), "create")
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

func (r *Reconciler) EnsureNotificationDeleted(ctx context.Context, UID string, owner kmeta.OwnerRefable, secret corev1.SecretKeySelector, project, bucket, notificationId string) (ops.OpsJobStatus, error) {
	return r.ensureNotificationJob(ctx, operations.NotificationArgs{
		UID:            UID,
		Image:          r.NotificationOpsImage,
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
func (r *Reconciler) deleteNotification(ctx context.Context, storage *v1alpha1.Storage) error {
	if storage.Status.NotificationID == "" {
		return nil
	}

	state, err := r.EnsureNotificationDeleted(ctx, string(storage.UID), storage, *storage.Spec.Secret, storage.Status.ProjectID, storage.Spec.Bucket, storage.Status.NotificationID)

	if state != ops.OpsJobCompleteSuccessful {
		return fmt.Errorf("Job %q has not completed yet", storage.Name)
	}

	// See if the pod exists or not...
	pod, err := ops.GetJobPod(ctx, r.KubeClientSet, storage.Namespace, string(storage.UID), "delete")
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

	r.Logger.Infof("Deleted Notification: %q", storage.Status.NotificationID)
	storage.Status.NotificationID = ""
	return nil
}

func addFinalizer(s *v1alpha1.Storage) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.Storage) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Storage) (*v1alpha1.Storage, error) {
	source, err := r.storageLister.Storages(desired.Namespace).Get(desired.Name)
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
	src, err := r.RunClientSet.EventsV1alpha1().Storages(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("Storage %q became ready after %v", source.Name, duration)

		if err := r.StatsReporter.ReportReady("Storage", source.Namespace, source.Name, duration); err != nil {
			logging.FromContext(ctx).Infof("failed to record ready for Storage, %v", err)
		}
	}

	return src, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.Storage) (*v1alpha1.Storage, bool, error) {
	storage, err := r.storageLister.Storages(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, false, err
	}

	// Don't modify the informers copy.
	existing := storage.DeepCopy()

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

	update, err := r.RunClientSet.EventsV1alpha1().Storages(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}

func (r *Reconciler) ensureNotificationJob(ctx context.Context, args operations.NotificationArgs) (ops.OpsJobStatus, error) {
	jobName := operations.NotificationJobName(args.Owner, args.Action)
	job, err := r.jobLister.Jobs(args.Owner.GetObjectMeta().GetNamespace()).Get(jobName)

	// If the resource doesn't exist, we'll create it
	if apierrs.IsNotFound(err) {
		r.Logger.Debugw("Job not found, creating with:", zap.Any("args", args))

		args.Image = r.NotificationOpsImage

		job, err = operations.NewNotificationOps(args)
		if err != nil {
			return ops.OpsJobCreateFailed, err
		}

		job, err := r.KubeClientSet.BatchV1().Jobs(args.Owner.GetObjectMeta().GetNamespace()).Create(job)
		if err != nil || job == nil {
			r.Logger.Debugw("Failed to create Job.", zap.Error(err))
			return ops.OpsJobCreateFailed, err
		}

		r.Logger.Debugw("Created Job.")
		return ops.OpsJobCreated, nil
	} else if err != nil {
		r.Logger.Debugw("Failed to get Job.", zap.Error(err))
		return ops.OpsJobGetFailed, err
		// TODO: Handle this case
		//	} else if !metav1.IsControlledBy(job, args.Owner) {
		//		return ops.OpsJobCreateFailed, fmt.Errorf("storage does not own job %q", jobName)
	}

	if ops.IsJobFailed(job) {
		return ops.OpsJobCompleteFailed, errors.New(ops.JobFailedMessage(job))
	}

	if ops.IsJobComplete(job) {
		r.Logger.Debugw("Job is complete.")
		return ops.OpsJobCompleteSuccessful, nil
	}

	r.Logger.Debug("Job still active.", zap.Any("job", job))
	return ops.OpsJobOngoing, nil
}
