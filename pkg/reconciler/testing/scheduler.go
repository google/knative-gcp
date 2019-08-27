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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// SchedulerOption enables further configuration of a Scheduler.
type SchedulerOption func(*v1alpha1.Scheduler)

// NewScheduler creates a Scheduler with SchedulerOptions
func NewScheduler(name, namespace string, so ...SchedulerOption) *v1alpha1.Scheduler {
	s := &v1alpha1.Scheduler{
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

func WithSchedulerBucket(bucket string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Spec.Bucket = bucket
	}
}

func WithSchedulerSink(gvk metav1.GroupVersionKind, name string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Spec.Sink = apisv1alpha1.Destination{
			ObjectReference: &corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

// WithInitSchedulerConditions initializes the Schedulers's conditions.
func WithInitSchedulerConditions(s *v1alpha1.Scheduler) {
	s.Status.InitializeConditions()
}

// WithSchedulerTopicNotReady marks the condition that the
// topic is not ready
func WithSchedulerTopicNotReady(reason, message string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkTopicNotReady(reason, message)
	}
}

// WithSchedulerTopicNotReady marks the condition that the
// topic is not ready
func WithSchedulerTopicReady(topicID string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkTopicReady()
		s.Status.TopicID = topicID
	}
}

// WithSchedulerPullSubscriptionNotReady marks the condition that the
// topic is not ready
func WithSchedulerPullSubscriptionNotReady(reason, message string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkPullSubscriptionNotReady(reason, message)
	}
}

// WithSchedulerPullSubscriptionNotReady marks the condition that the
// topic is not ready
func WithSchedulerPullSubscriptionReady() SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkPullSubscriptionReady()
	}
}

// WithSchedulerNotificationNotReady marks the condition that the
// GCS Notification is not ready.
func WithSchedulerNotificationNotReady(reason, message string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkNotificationNotReady(reason, message)
	}
}

// WithSchedulerNotificationReady marks the condition that the GCS
// Notification is ready.
func WithSchedulerNotificationReady() SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.MarkNotificationReady()
	}
}

// WithSchedulerSinkURI sets the status for sink URI
func WithSchedulerSinkURI(url *apis.URL) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.SinkURI = url
	}
}

// WithSchedulerNotificationId sets the status for Notification ID
func WithSchedulerNotificationID(notificationID string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.NotificationID = notificationID
	}
}

// WithSchedulerProjectId sets the status for Project ID
func WithSchedulerProjectID(notificationID string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Status.ProjectID = notificationID
	}
}

func WithSchedulerFinalizers(finalizers ...string) SchedulerOption {
	return func(s *v1alpha1.Scheduler) {
		s.Finalizers = finalizers
	}
}
