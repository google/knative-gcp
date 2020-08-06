/*
Copyright 2020 Google LLC.

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

package v1

import (
	"time"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/duck"
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	"github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

// CloudStorageSourceOption enables further configuration of a CloudStorageSource.
type CloudStorageSourceOption func(*v1.CloudStorageSource)

// NewCloudStorageSource creates a CloudStorageSource with CloudStorageSourceOptions
func NewCloudStorageSource(name, namespace string, so ...CloudStorageSourceOption) *v1.CloudStorageSource {
	s := &v1.CloudStorageSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-storage-uid",
			Annotations: map[string]string{
				duck.ClusterNameAnnotation: testing.FakeClusterName,
			},
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithCloudStorageSourceBucket(bucket string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Spec.Bucket = bucket
	}
}

func WithCloudStorageSourceProject(project string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Spec.Project = project
	}
}

func WithCloudStorageSourceEventTypes(eventTypes []string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Spec.EventTypes = eventTypes
	}
}

func WithCloudStorageSourceSink(gvk metav1.GroupVersionKind, name string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: reconcilertesting.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudStorageSourceSinkDestination(sink duckv1.Destination) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Spec.Sink = sink
	}
}

// WithInitCloudStorageSourceConditions initializes the CloudStorageSources's conditions.
func WithInitCloudStorageSourceConditions(s *v1.CloudStorageSource) {
	s.Status.InitializeConditions()
}

func WithCloudStorageSourceWorkloadIdentityFailed(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

func WithCloudStorageSourceServiceAccount(kServiceAccount string) CloudStorageSourceOption {
	return func(ps *v1.CloudStorageSource) {
		ps.Spec.ServiceAccountName = kServiceAccount
	}
}

// WithCloudStorageSourceTopicFailed marks the condition that the
// topic is False.
func WithCloudStorageSourceTopicFailed(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkTopicFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudStorageSourceTopicUnknown marks the condition that the
// topic is Unknown.
func WithCloudStorageSourceTopicUnknown(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkTopicUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudStorageSourceTopicNotReady marks the condition that the
// topic is not ready.
func WithCloudStorageSourceTopicReady(topicID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkTopicReady(s.ConditionSet())
		s.Status.TopicID = topicID
	}
}

// WithCloudStorageSourceTopicDeleted is a wrapper to indicate that the
// topic is deleted. Inside the function, we still mark the status of topic to be ready,
// as the status of topic is unchanged if the deletion is successful. We do not set the
// topicID because the topicID is set to empty when deleting the topic.
func WithCloudStorageSourceTopicDeleted(s *v1.CloudStorageSource) {
	s.Status.MarkTopicReady(s.ConditionSet())
}

func WithCloudStorageSourceTopicID(topicID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.TopicID = topicID
	}
}

// WithCloudStorageSourcePullSubscriptionFailed marks the condition that the
// status of topic is False.
func WithCloudStorageSourcePullSubscriptionFailed(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudStorageSourcePullSubscriptionUnknown marks the condition that the
// status of topic is Unknown.
func WithCloudStorageSourcePullSubscriptionUnknown(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudStorageSourcePullSubscriptionReady marks the condition that the
// pullSubscription is ready.
func WithCloudStorageSourcePullSubscriptionReady(s *v1.CloudStorageSource) {
	s.Status.MarkPullSubscriptionReady(s.ConditionSet())
}

// WithCloudStorageSourcePullSubscriptionDeleted a wrapper to indicate that the
// pullSubscription is deleted. Inside the function, we still mark the status of
// pullSubscription to be ready, as the status of pullSubscription is unchanged
// if the deletion is successful.
func WithCloudStorageSourcePullSubscriptionDeleted(s *v1.CloudStorageSource) {
	s.Status.MarkPullSubscriptionReady(s.ConditionSet())
}

// WithCloudStorageSourceNotificationNotReady marks the condition that the
// GCS Notification is not ready.
func WithCloudStorageSourceNotificationNotReady(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkNotificationNotReady(reason, message)
	}
}

// WithCloudStorageSourceNotificationNotReady marks the condition that the
// GCS Notification is unknown.
func WithCloudStorageSourceNotificationUnknown(reason, message string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkNotificationUnknown(reason, message)
	}
}

// WithCloudStorageSourceNotificationReady marks the condition that the GCS
// Notification is ready.
func WithCloudStorageSourceNotificationReady(notificationID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkNotificationReady(notificationID)
	}
}

// WithCloudStorageSourceNotificationDeleted a wrapper to indicate that the
// notification is deleted. Inside the function, we still mark the status of
// notification to be ready, as the status of notification is unchanged if
// the deletion is successful.
func WithCloudStorageSourceNotificationDeleted(notificationID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.MarkNotificationReady(notificationID)
	}
}

// WithCloudStorageSourceSinkURI sets the status for sink URI.
func WithCloudStorageSourceSinkURI(url *apis.URL) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.SinkURI = url
	}
}

// WithCloudStorageSourceNotificationId sets the status for Notification ID.
func WithCloudStorageSourceNotificationID(notificationID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.NotificationID = notificationID
	}
}

// WithCloudStorageSourceProjectId sets the status for Project ID.
func WithCloudStorageSourceProjectID(projectID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.ProjectID = projectID
	}
}

func WithCloudStorageSourceSubscriptionID(subscriptionID string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.SubscriptionID = subscriptionID
	}
}

func WithCloudStorageSourceStatusObservedGeneration(generation int64) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.Status.Status.ObservedGeneration = generation
	}
}

func WithCloudStorageSourceObjectMetaGeneration(generation int64) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.ObjectMeta.Generation = generation
	}
}

func WithDeletionTimestamp(s *v1.CloudStorageSource) {
	ts := metav1.NewTime(time.Unix(1e9, 0))
	s.DeletionTimestamp = &ts
}

func WithCloudStorageSourceAnnotations(Annotations map[string]string) CloudStorageSourceOption {
	return func(s *v1.CloudStorageSource) {
		s.ObjectMeta.Annotations = Annotations
	}
}

func WithCloudStorageSourceSetDefaults(s *v1.CloudStorageSource) {
	s.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}
