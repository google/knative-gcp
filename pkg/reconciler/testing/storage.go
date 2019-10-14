/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// StorageOption enables further configuration of a Storage.
type StorageOption func(*v1alpha1.Storage)

// NewStorage creates a Storage with StorageOptions
func NewStorage(name, namespace string, so ...StorageOption) *v1alpha1.Storage {
	s := &v1alpha1.Storage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-storage-uid",
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithStorageBucket(bucket string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Spec.Bucket = bucket
	}
}

func WithStorageEventTypes(eventTypes []string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Spec.EventTypes = eventTypes
	}
}

func WithStorageSink(gvk metav1.GroupVersionKind, name string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Spec.Sink = apisv1alpha1.Destination{
			ObjectReference: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

// WithInitStorageConditions initializes the Storages's conditions.
func WithInitStorageConditions(s *v1alpha1.Storage) {
	s.Status.InitializeConditions()
}

// WithStorageTopicNotReady marks the condition that the
// topic is not ready
func WithStorageTopicNotReady(reason, message string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkTopicNotReady(reason, message)
	}
}

// WithStorageTopicNotReady marks the condition that the
// topic is not ready
func WithStorageTopicReady(topicID string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

// WithStoragePullSubscriptionNotReady marks the condition that the
// topic is not ready
func WithStoragePullSubscriptionNotReady(reason, message string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkPullSubscriptionNotReady(reason, message)
	}
}

// WithStoragePullSubscriptionNotReady marks the condition that the
// topic is not ready
func WithStoragePullSubscriptionReady() StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkPullSubscriptionReady()
	}
}

// WithStorageNotificationNotReady marks the condition that the
// GCS Notification is not ready.
func WithStorageNotificationNotReady(reason, message string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkNotificationNotReady(reason, message)
	}
}

// WithStorageNotificationReady marks the condition that the GCS
// Notification is ready.
func WithStorageNotificationReady() StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.MarkNotificationReady()
	}
}

// WithStorageSinkURI sets the status for sink URI
func WithStorageSinkURI(url *apis.URL) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.SinkURI = url
	}
}

// WithStorageNotificationId sets the status for Notification ID
func WithStorageNotificationID(notificationID string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.NotificationID = notificationID
	}
}

// WithStorageProjectId sets the status for Project ID
func WithStorageProjectID(projectID string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.ProjectID = projectID
	}
}

func WithStorageFinalizers(finalizers ...string) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Finalizers = finalizers
	}
}

func WithStorageStatusObservedGeneration(generation int64) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithStorageObjectMetaGeneration(generation int64) StorageOption {
	return func(s *v1alpha1.Storage) {
		s.ObjectMeta.Generation = generation
	}
}

func WithDeletionTimestamp() StorageOption {
	return func(s *v1alpha1.Storage) {
		ts := metav1.NewTime(time.Unix(1e9, 0))
		s.DeletionTimestamp = &ts
	}
}
