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
	"fmt"
	"time"

	"go.uber.org/zap"
	gstatus "google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	. "cloud.google.com/go/storage"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/reconciler/events/storage/resources"
	"github.com/google/knative-gcp/pkg/reconciler/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	finalizerName = controllerAgentName

	resourceGroup = "cloudstoragesources.events.cloud.google.com"
)

var (
	// Mapping of the storage source CloudEvent types to google storage types.
	storageEventTypes = map[string]string{
		v1alpha1.CloudStorageSourceFinalize:       "OBJECT_FINALIZE",
		v1alpha1.CloudStorageSourceArchive:        "OBJECT_ARCHIVE",
		v1alpha1.CloudStorageSourceDelete:         "OBJECT_DELETE",
		v1alpha1.CloudStorageSourceMetadataUpdate: "OBJECT_METADATA_UPDATE",
	}
)

// Reconciler is the controller implementation for Google Cloud Storage (GCS) event
// notifications.
type Reconciler struct {
	*pubsub.PubSubBase

	// storageLister for reading storages.
	storageLister listers.CloudStorageSourceLister

	// createClientFn is the function used to create the Storage client that interacts with GCS.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gstorage.CreateFn
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the CloudStorageSource resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Invalid resource key")
		return nil
	}

	// Get the CloudStorageSource resource with this namespace/name
	original, err := r.storageLister.CloudStorageSources(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The CloudStorageSource resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Desugar().Error("CloudStorageSource in work queue no longer exists")
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
		logging.FromContext(ctx).Desugar().Warn("Failed to update CloudStorageSource finalizers", zap.Error(fErr))
		r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update finalizers for CloudStorageSource %q: %v", storage.Name, fErr)
		return fErr
	} else if updated {
		// There was a difference and updateFinalizers said it updated and did not return an error.
		r.Recorder.Eventf(storage, corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q finalizers", storage.Name)
	}

	if equality.Semantic.DeepEqual(original.Status, storage.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.

	} else if _, uErr := r.updateStatus(ctx, storage); uErr != nil {
		logging.FromContext(ctx).Desugar().Warn("Failed to update CloudStorageSource status", zap.Error(uErr))
		r.Recorder.Eventf(storage, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for CloudStorageSource %q: %v", storage.Name, uErr)
		return uErr
	} else if reconcileErr == nil {
		// There was a difference and updateStatus did not return an error.
		r.Recorder.Eventf(storage, corev1.EventTypeNormal, "Updated", "Updated CloudStorageSource %q", storage.Name)
	}
	if reconcileErr != nil {
		r.Recorder.Event(storage, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, storage *v1alpha1.CloudStorageSource) error {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("storage", storage)))

	storage.Status.InitializeConditions()

	// See if the source has been deleted.
	if storage.DeletionTimestamp != nil {
		logging.FromContext(ctx).Desugar().Debug("Deleting CloudStorageSource notification")
		if err := r.deleteNotification(ctx, storage); err != nil {
			storage.Status.MarkNotificationNotReady("NotificationDeleteFailed", "Failed to delete CloudStorageSource notification: %s", err.Error())
			return err
		}
		storage.Status.MarkNotificationNotReady("NotificationDeleted", "Successfully deleted CloudStorageSource notification: %s", storage.Status.NotificationID)

		if err := r.PubSubBase.DeletePubSub(ctx, storage); err != nil {
			return err
		}

		// Only set the notificationID to empty after we successfully deleted the PubSub resources.
		// Otherwise, we may leak them.
		storage.Status.NotificationID = ""
		removeFinalizer(storage)
		return nil
	}

	// Ensure that there's finalizer there, since we're about to attempt to
	// change external state with the topic, so we need to clean it up.
	addFinalizer(storage)

	topic := resources.GenerateTopicName(storage)
	_, _, err := r.PubSubBase.ReconcilePubSub(ctx, storage, topic, resourceGroup)
	if err != nil {
		return err
	}

	notification, err := r.reconcileNotification(ctx, storage)
	if err != nil {
		storage.Status.MarkNotificationNotReady("NotificationReconcileFailed", "Failed to reconcile CloudStorageSource notification: %s", err.Error())
		return err
	}
	storage.Status.MarkNotificationReady(notification)

	return nil
}

func (r *Reconciler) reconcileNotification(ctx context.Context, storage *v1alpha1.CloudStorageSource) (string, error) {
	if storage.Status.ProjectID == "" {
		projectID, err := utils.ProjectID(storage.Spec.Project)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to find project id", zap.Error(err))
			return "", err
		}
		// Set the projectID in the status.
		storage.Status.ProjectID = projectID
	}

	client, err := r.createClientFn(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudStorageSource client", zap.Error(err))
		return "", err
	}
	defer client.Close()

	// Load the Bucket.
	bucket := client.Bucket(storage.Spec.Bucket)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to fetch existing notifications", zap.Error(err))
		return "", err
	}

	// If the notification does exist, then return its ID.
	if existing, ok := notifications[storage.Status.NotificationID]; ok {
		return existing.ID, nil
	}

	// If the notification does not exist, then create it.

	// Add our own converter type as a customAttribute.
	customAttributes := map[string]string{
		converters.KnativeGCPConverter: converters.CloudStorageConverter,
	}

	nc := &Notification{
		TopicProjectID:   storage.Status.ProjectID,
		TopicID:          storage.Status.TopicID,
		PayloadFormat:    JSONPayload,
		EventTypes:       r.toCloudStorageSourceEventTypes(storage.Spec.EventTypes),
		ObjectNamePrefix: storage.Spec.ObjectNamePrefix,
		CustomAttributes: customAttributes,
	}

	notification, err := bucket.AddNotification(ctx, nc)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudStorageSource notification", zap.Error(err))
		return "", err
	}
	return notification.ID, nil
}

func (r *Reconciler) toCloudStorageSourceEventTypes(eventTypes []string) []string {
	storageTypes := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		storageTypes = append(storageTypes, storageEventTypes[eventType])
	}
	return storageTypes
}

// deleteNotification looks at the status.NotificationID and if non-empty,
// hence indicating that we have created a notification successfully
// in the CloudStorageSource, remove it.
func (r *Reconciler) deleteNotification(ctx context.Context, storage *v1alpha1.CloudStorageSource) error {
	if storage.Status.NotificationID == "" {
		return nil
	}

	// At this point the project should have been populated.
	// Querying CloudStorageSource as the notification could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudStorageSource client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Load the Bucket.
	bucket := client.Bucket(storage.Spec.Bucket)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to fetch existing notifications", zap.Error(err))
		return err
	}

	// This is bit wonky because, we could always just try to delete, but figuring out
	// if an error returned is NotFound seems to not really work, so, we'll try
	// checking first the list and only then deleting.
	if existing, ok := notifications[storage.Status.NotificationID]; ok {
		logging.FromContext(ctx).Desugar().Debug("Found existing notification", zap.Any("notification", existing))
		err = bucket.DeleteNotification(ctx, storage.Status.NotificationID)
		if err == nil {
			logging.FromContext(ctx).Desugar().Debug("Deleted Notification", zap.String("notificationId", storage.Status.NotificationID))
			return nil
		}
		if st, ok := gstatus.FromError(err); !ok {
			logging.FromContext(ctx).Desugar().Error("Failed from CloudStorageSource client while deleting CloudStorageSource notification", zap.String("notificationId", storage.Status.NotificationID), zap.Error(err))
			return err
		} else if st.Code() != codes.NotFound {
			logging.FromContext(ctx).Desugar().Error("Failed to delete CloudStorageSource notification", zap.String("notificationId", storage.Status.NotificationID), zap.Error(err))
			return err
		}
	}
	return nil
}

func addFinalizer(s *v1alpha1.CloudStorageSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func removeFinalizer(s *v1alpha1.CloudStorageSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.CloudStorageSource) (*v1alpha1.CloudStorageSource, error) {
	source, err := r.storageLister.CloudStorageSources(desired.Namespace).Get(desired.Name)
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
	src, err := r.RunClientSet.EventsV1alpha1().CloudStorageSources(desired.Namespace).UpdateStatus(existing)

	if err == nil && becomesReady {
		// TODO compute duration since last non-ready. See https://github.com/google/knative-gcp/issues/455.
		duration := time.Since(src.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Desugar().Info("CloudStorageSource became ready", zap.Any("after", duration))
		r.Recorder.Event(source, corev1.EventTypeNormal, "ReadinessChanged", fmt.Sprintf("CloudStorageSource %q became ready", source.Name))
		if metricErr := r.StatsReporter.ReportReady("CloudStorageSource", source.Namespace, source.Name, duration); metricErr != nil {
			logging.FromContext(ctx).Desugar().Error("Failed to record ready for CloudStorageSource", zap.Error(metricErr))
		}
	}

	return src, err
}

// updateFinalizers is a generic method for future compatibility with a
// reconciler SDK.
func (r *Reconciler) updateFinalizers(ctx context.Context, desired *v1alpha1.CloudStorageSource) (*v1alpha1.CloudStorageSource, bool, error) {
	storage, err := r.storageLister.CloudStorageSources(desired.Namespace).Get(desired.Name)
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

	update, err := r.RunClientSet.EventsV1alpha1().CloudStorageSources(existing.Namespace).Patch(existing.Name, types.MergePatchType, patch)
	return update, true, err
}
