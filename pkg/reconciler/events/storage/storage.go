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

	"go.uber.org/zap"

	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	. "cloud.google.com/go/storage"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	cloudstoragesourcereconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/events/v1/cloudstoragesource"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1"
	gstorage "github.com/google/knative-gcp/pkg/gclient/storage"
	"github.com/google/knative-gcp/pkg/reconciler/events/storage/resources"
	"github.com/google/knative-gcp/pkg/reconciler/identity"
	"github.com/google/knative-gcp/pkg/reconciler/intevents"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	resourceGroup = "cloudstoragesources.events.cloud.google.com"

	deleteNotificationFailed     = "NotificationDeleteFailed"
	deletePubSubFailed           = "PubSubDeleteFailed"
	deleteWorkloadIdentityFailed = "WorkloadIdentityDeleteFailed"
	reconciledNotificationFailed = "NotificationReconcileFailed"
	reconciledPubSubFailed       = "PubSubReconcileFailed"
	reconciledSuccessReason      = "CloudStorageSourceReconciled"
	workloadIdentityFailed       = "WorkloadIdentityReconcileFailed"
)

var (
	// Mapping of the storage source CloudEvent types to google storage types.
	storageEventTypes = map[string]string{
		schemasv1.CloudStorageObjectFinalizedEventType:       "OBJECT_FINALIZE",
		schemasv1.CloudStorageObjectArchivedEventType:        "OBJECT_ARCHIVE",
		schemasv1.CloudStorageObjectDeletedEventType:         "OBJECT_DELETE",
		schemasv1.CloudStorageObjectMetadataUpdatedEventType: "OBJECT_METADATA_UPDATE",
	}
)

// Reconciler is the controller implementation for Google Cloud Storage (GCS) event
// notifications.
type Reconciler struct {
	*intevents.PubSubBase
	// identity reconciler for reconciling workload identity.
	*identity.Identity
	// storageLister for reading storages.
	storageLister listers.CloudStorageSourceLister

	// createClientFn is the function used to create the Storage client that interacts with GCS.
	// This is needed so that we can inject a mock client for UTs purposes.
	createClientFn gstorage.CreateFn
}

// Check that our Reconciler implements Interface.
var _ cloudstoragesourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, storage *v1.CloudStorageSource) reconciler.Event {
	ctx = logging.WithLogger(ctx, r.Logger.With(zap.Any("storage", storage)))

	storage.Status.InitializeConditions()
	storage.Status.ObservedGeneration = storage.Generation

	// If ServiceAccountName is provided, reconcile workload identity.
	if storage.Spec.ServiceAccountName != "" {
		if _, err := r.Identity.ReconcileWorkloadIdentity(ctx, storage.Spec.Project, storage); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, workloadIdentityFailed, "Failed to reconcile CloudStorageSource workload identity: %s", err.Error())
		}
	}

	topic := resources.GenerateTopicName(storage)
	_, _, err := r.PubSubBase.ReconcilePubSub(ctx, storage, topic, resourceGroup)
	if err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledPubSubFailed, "Failed to reconcile CloudStorageSource PubSub: %s", err.Error())
	}

	notification, err := r.reconcileNotification(ctx, storage)
	if err != nil {
		storage.Status.MarkNotificationNotReady(reconciledNotificationFailed, "Failed to reconcile CloudStorageSource notification: %s", err.Error())
		return reconciler.NewEvent(corev1.EventTypeWarning, reconciledNotificationFailed, "Failed to reconcile CloudStorageSource notification: %s", err.Error())
	}
	storage.Status.MarkNotificationReady(notification)

	return reconciler.NewEvent(corev1.EventTypeNormal, reconciledSuccessReason, `CloudStorageSource reconciled: "%s/%s"`, storage.Namespace, storage.Name)
}

func (r *Reconciler) reconcileNotification(ctx context.Context, storage *v1.CloudStorageSource) (string, error) {
	if storage.Status.ProjectID == "" {
		projectID, err := utils.ProjectIDOrDefault(storage.Spec.Project)
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
	//Check whether Bucket exists or not
	if _, err := bucket.Attrs(ctx); err != nil {
		if err == ErrBucketNotExist {
			logging.FromContext(ctx).Desugar().Error("Bucket doesn't exist", zap.String("bucketName", storage.Spec.Bucket), zap.Error(err))
			return "", err
		}
		logging.FromContext(ctx).Desugar().Error("Failed to fetch attrs of bucket", zap.String("bucketName", storage.Spec.Bucket), zap.Error(err))
		return "", err
	}

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

	nc := &Notification{
		TopicProjectID:   storage.Status.ProjectID,
		TopicID:          storage.Status.TopicID,
		PayloadFormat:    JSONPayload,
		EventTypes:       r.toCloudStorageSourceEventTypes(storage.Spec.EventTypes),
		ObjectNamePrefix: storage.Spec.ObjectNamePrefix,
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
func (r *Reconciler) deleteNotification(ctx context.Context, storage *v1.CloudStorageSource) error {
	if storage.Status.NotificationID == "" {
		return nil
	}

	// At this point the project should have been populated.
	// Querying CloudStorageSource as the notification could have been deleted outside the cluster (e.g, through gcloud).
	client, err := r.createClientFn(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to create CloudStorageSource client", zap.Error(err))
		storage.Status.MarkNotificationUnknown(deleteNotificationFailed, "Failed to create CloudStorageSource client: %s", err.Error())
		return err
	}
	defer client.Close()

	// Load the Bucket.
	bucket := client.Bucket(storage.Spec.Bucket)

	// Check whether bucket exists or not
	if _, err := bucket.Attrs(ctx); err != nil {
		// If the bucket was already deleted, then we should  proceed.
		if err == ErrBucketNotExist {
			logging.FromContext(ctx).Desugar().Info("Bucket does not exist.", zap.String("bucketName", storage.Spec.Bucket), zap.Error(err))
			return nil
		}
		logging.FromContext(ctx).Desugar().Error("Failed to fetch attrs of bucket", zap.String("bucketName", storage.Spec.Bucket), zap.Error(err))
		storage.Status.MarkNotificationUnknown(deleteNotificationFailed, "Failed to fetch attrs of bucket: %s", err.Error())
		return err
	}

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Failed to fetch existing notifications", zap.Error(err))
		storage.Status.MarkNotificationUnknown(deleteNotificationFailed, "Failed to fetch existing notifications: %s", err.Error())
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
			storage.Status.MarkNotificationUnknown(deleteNotificationFailed, "Failed from CloudStorageSource client while deleting CloudStorageSource notification: %s", err.Error())
			return err
		} else if st.Code() != codes.NotFound {
			logging.FromContext(ctx).Desugar().Error("Failed to delete CloudStorageSource notification", zap.String("notificationId", storage.Status.NotificationID), zap.Error(err))
			storage.Status.MarkNotificationUnknown(deleteNotificationFailed, "Failed to delete CloudStorageSource notification: %s", err.Error())
			return err
		}
	}
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, storage *v1.CloudStorageSource) reconciler.Event {
	// If k8s ServiceAccount exists, binds to the default GCP ServiceAccount, and it only has one ownerReference,
	// remove the corresponding GCP ServiceAccount iam policy binding.
	// No need to delete k8s ServiceAccount, it will be automatically handled by k8s Garbage Collection.
	if storage.Spec.ServiceAccountName != "" {
		if err := r.Identity.DeleteWorkloadIdentity(ctx, storage.Spec.Project, storage); err != nil {
			return reconciler.NewEvent(corev1.EventTypeWarning, deleteWorkloadIdentityFailed, "Failed to delete CloudStorageSource workload identity: %s", err.Error())
		}
	}

	logging.FromContext(ctx).Desugar().Debug("Deleting CloudStorageSource notification")
	if err := r.deleteNotification(ctx, storage); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deleteNotificationFailed, "Failed to delete CloudStorageSource notification: %s", err.Error())
	}

	if err := r.PubSubBase.DeletePubSub(ctx, storage); err != nil {
		return reconciler.NewEvent(corev1.EventTypeWarning, deletePubSubFailed, "Failed to delete CloudStorageSource PubSub: %s", err.Error())
	}

	// ok to remove finalizer.
	return nil
}
