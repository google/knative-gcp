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

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"cloud.google.com/go/pubsub"
	storageClient "cloud.google.com/go/storage"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubsourcev1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	clientset "github.com/google/knative-gcp/pkg/client/clientset/versioned"
	pubsubsourceclientset "github.com/google/knative-gcp/pkg/client/clientset/versioned"
	pubsubsourceinformers "github.com/google/knative-gcp/pkg/client/informers/externalversions/pubsub/v1alpha1"
	listers "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"
	"github.com/google/knative-gcp/pkg/duck"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/storage/resources"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "PullSubscriptions"

	finalizerName = controllerAgentName
)

// Reconciler is the controller implementation for Google Cloud Storage (GCS) event
// notifications.
type Reconciler struct {
	*reconciler.Base

	// gcssourceclientset is a clientset for our own API group
	storageclientset clientset.Interface
	storageLister    listers.StorageLister

	// For dealing with
	pubsubClient           pubsubsourceclientset.Interface
	pubsubInformer         pubsubsourceinformers.PullSubscriptionInformer
	pullSubscriptionLister pubsublisters.PullSubscriptionLister
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
	if errors.IsNotFound(err) {
		// The Storage resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("storage '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	csr := original.DeepCopy()

	err = c.reconcileStorageSource(ctx, csr)

	if equality.Semantic.DeepEqual(original.Status, csr.Status) &&
		equality.Semantic.DeepEqual(original.ObjectMeta, csr.ObjectMeta) {
		// If we didn't change anything (status or finalizers) then don't
		// call update.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(ctx, csr); err != nil {
		c.Logger.Warn("Failed to update Storage Source status", zap.Error(err))
		return err
	}
	return err
}

func (c *Reconciler) reconcileStorageSource(ctx context.Context, csr *v1alpha1.Storage) error {
	// See if the source has been deleted.
	deletionTimestamp := csr.DeletionTimestamp

	// First try to resolve the sink, and if not found mark as not resolved.
	uri, err := duck.GetSinkURI(ctx, c.DynamicClientSet, &csr.Spec.Sink, csr.Namespace)
	if err != nil {
		// TODO: Update status appropriately
		//		csr.Status.MarkNoSink("NotFound", "%s", err)
		c.Logger.Infof("Couldn't resolve Sink URI: %s", err)
		if deletionTimestamp == nil {
			return err
		}
		// we don't care about the URI if we're deleting, so carry on...
		uri = ""
	}
	c.Logger.Infof("Resolved Sink URI to %q", uri)

	if deletionTimestamp != nil {
		err := c.deleteNotification(csr)
		if err != nil {
			c.Logger.Infof("Unable to delete the Notification: %s", err)
			return err
		}
		err = c.deleteTopic(csr.Spec.Project, csr.Status.Topic)
		if err != nil {
			c.Logger.Infof("Unable to delete the Topic: %s", err)
			return err
		}
		csr.Status.Topic = ""
		c.removeFinalizer(csr)
		return nil
	}

	csr.Status.InitializeConditions()

	err = c.reconcileTopic(csr)
	if err != nil {
		c.Logger.Infof("Failed to reconcile topic %s", err)
		csr.Status.MarkPubSubTopicNotReady(fmt.Sprintf("Failed to create GCP PubSub Topic: %s", err), "")
		return err
	}

	csr.Status.MarkPubSubTopicReady()

	c.addFinalizer(csr)

	csr.Status.SinkURI = uri

	// Make sure PullSubscription is in the state we expect it to be in.
	pubsub, err := c.reconcilePullSubscription(csr)
	if err != nil {
		// TODO: Update status appropriately
		c.Logger.Infof("Failed to reconcile PullSubscription Source: %s", err)
		csr.Status.MarkPullSubscriptionNotReady(fmt.Sprintf("Failed to create GCP PullSubscription Source: %s", err), "")
		return err
	}
	c.Logger.Infof("Reconciled pubsub source: %+v", pubsub)
	c.Logger.Infof("using %q as a cluster internal sink", pubsub.Status.SinkURI)

	// Check to see if pubsub source is ready
	if !pubsub.Status.IsReady() {
		c.Logger.Infof("GCP PullSubscription Source is not ready yet")
		csr.Status.MarkPullSubscriptionNotReady("underlying GCP PullSubscription Source is not ready", "")
	} else {
		csr.Status.MarkPullSubscriptionReady()
	}

	notification, err := c.reconcileNotification(csr)
	if err != nil {
		// TODO: Update status with this...
		c.Logger.Infof("Failed to reconcile Storage Notification: %s", err)
		csr.Status.MarkGCSNotReady(fmt.Sprintf("Failed to create Storage notification: %s", err), "")
		return err
	}

	csr.Status.MarkGCSReady()

	c.Logger.Infof("Reconciled Storage notification: %+v", notification)
	csr.Status.NotificationID = notification.ID
	return nil
}

func (c *Reconciler) reconcilePullSubscription(csr *v1alpha1.Storage) (*pubsubsourcev1alpha1.PullSubscription, error) {
	pubsubClient := c.pubsubClient.PubsubV1alpha1().PullSubscriptions(csr.Namespace)
	existing, err := pubsubClient.Get(csr.Name, v1.GetOptions{})
	if err == nil {
		// TODO: Handle any updates...
		c.Logger.Infof("Found existing pubsubsource: %+v", existing)
		return existing, nil
	}
	if errors.IsNotFound(err) {
		pubsub := resources.MakePullSubscription(csr, "testing")
		c.Logger.Infof("Creating service %+v", pubsub)
		return pubsubClient.Create(pubsub)
	}
	return nil, err
}

func (c *Reconciler) reconcileNotification(storage *v1alpha1.Storage) (*storageClient.Notification, error) {
	ctx := context.Background()
	sc, err := storageClient.NewClient(ctx)
	if err != nil {
		c.Logger.Infof("Failed to create storage client: %s", err)
		return nil, err
	}

	bucket := sc.Bucket(storage.Spec.Bucket)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		c.Logger.Infof("Failed to fetch existing notifications: %s", err)
		return nil, err
	}

	if storage.Status.NotificationID != "" {
		if existing, ok := notifications[storage.Status.NotificationID]; ok {
			c.Logger.Infof("Found existing notification: %+v", existing)
			return existing, nil
		}
	}

	customAttributes := make(map[string]string)
	for k, v := range storage.Spec.CustomAttributes {
		customAttributes[k] = v
	}

	// Add our own event type here...
	customAttributes["ce-type"] = "google.storage"

	c.Logger.Infof("Creating a notification on bucket %s", storage.Spec.Bucket)
	notification, err := bucket.AddNotification(ctx, &storageClient.Notification{
		TopicProjectID:   storage.Spec.Project,
		TopicID:          storage.Status.Topic,
		PayloadFormat:    storageClient.JSONPayload,
		EventTypes:       storage.Spec.EventTypes,
		ObjectNamePrefix: storage.Spec.ObjectNamePrefix,
		CustomAttributes: customAttributes,
	})

	if err != nil {
		c.Logger.Infof("Failed to create Notification: %s", err)
		return nil, err
	}
	c.Logger.Infof("Created Notification %q", notification.ID)

	return notification, nil
}

func (c *Reconciler) reconcileTopic(csr *v1alpha1.Storage) error {
	if csr.Status.Topic == "" {
		c.Logger.Infof("No topic found in status, creating a unique one")
		// Create a UUID for the topic. prefix with storage- to make it conformant.
		csr.Status.Topic = fmt.Sprintf("storage-%s", uuid.New().String())
	}

	ctx := context.Background()
	psc, err := pubsub.NewClient(ctx, csr.Spec.Project)
	if err != nil {
		return err
	}
	topic := psc.Topic(csr.Status.Topic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		c.Logger.Infof("Failed to check for topic %q existence : %s", csr.Status.Topic, err)
		return err
	}
	if exists {
		c.Logger.Infof("Topic %q exists already", csr.Status.Topic)
		return nil
	}

	c.Logger.Infof("Creating topic %q", csr.Status.Topic)
	newTopic, err := psc.CreateTopic(ctx, csr.Status.Topic)
	if err != nil {
		c.Logger.Infof("Failed to create topic %q : %s", csr.Status.Topic, err)
		return err
	}
	c.Logger.Infof("Created topic %q : %+v", csr.Status.Topic, newTopic)
	return nil
}

func (c *Reconciler) deleteTopic(project string, topic string) error {
	// No topic, no delete...
	if topic == "" {
		return nil
	}
	ctx := context.Background()
	psc, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return err
	}
	t := psc.Topic(topic)
	err = t.Delete(context.Background())
	if err == nil {
		c.Logger.Infof("Deleted topic %q", topic)
		return nil
	}

	if st, ok := gstatus.FromError(err); !ok {
		c.Logger.Infof("Unknown error from the pubsub client: %s", err)
		return err
	} else if st.Code() != codes.NotFound {
		return err
	}
	return nil
}

// deleteNotification looks at the status.NotificationID and if non-empty
// hence indicating that we have created a notification successfully
// in the Storage, remove it.
func (c *Reconciler) deleteNotification(storage *v1alpha1.Storage) error {
	if storage.Status.NotificationID == "" {
		return nil
	}
	ctx := context.Background()
	sc, err := storageClient.NewClient(ctx)
	if err != nil {
		c.Logger.Infof("Failed to create storage client: %s", err)
		return err
	}

	bucket := sc.Bucket(storage.Spec.Bucket)
	c.Logger.Infof("Deleting notification as: %q", storage.Status.NotificationID)
	err = bucket.DeleteNotification(ctx, storage.Status.NotificationID)
	if err == nil {
		c.Logger.Infof("Deleted Notification: %q", storage.Status.NotificationID)
		return nil
	}

	if st, ok := gstatus.FromError(err); !ok {
		c.Logger.Infof("Unknown error from the cloud storage client: %s", err)
		return err
	} else if st.Code() != codes.NotFound {
		return err
	}
	return nil
}

func (c *Reconciler) addFinalizer(csr *v1alpha1.Storage) {
	finalizers := sets.NewString(csr.Finalizers...)
	finalizers.Insert(finalizerName)
	csr.Finalizers = finalizers.List()
}

func (c *Reconciler) removeFinalizer(csr *v1alpha1.Storage) {
	finalizers := sets.NewString(csr.Finalizers...)
	finalizers.Delete(finalizerName)
	csr.Finalizers = finalizers.List()
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
