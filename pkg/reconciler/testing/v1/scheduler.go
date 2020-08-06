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

	"github.com/google/knative-gcp/pkg/reconciler/testing"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
)

// CloudSchedulerSourceOption enables further configuration of a CloudSchedulerSource.
type CloudSchedulerSourceOption func(*v1.CloudSchedulerSource)

// NewCloudSchedulerSource creates a CloudSchedulerSource with CloudSchedulerSourceOptions
func NewCloudSchedulerSource(name, namespace string, so ...CloudSchedulerSourceOption) *v1.CloudSchedulerSource {
	s := &v1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-scheduler-uid",
		},
	}
	for _, opt := range so {
		opt(s)
	}
	return s
}

func WithCloudSchedulerSourceSink(gvk metav1.GroupVersionKind, name string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: testing.ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudSchedulerSourceLocation(location string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Spec.Location = location
	}
}

func WithCloudSchedulerSourceProject(project string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Spec.Project = project
	}
}

func WithCloudSchedulerSourceSchedule(schedule string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Spec.Schedule = schedule
	}
}

func WithCloudSchedulerSourceServiceAccount(kServiceAccount string) CloudSchedulerSourceOption {
	return func(ps *v1.CloudSchedulerSource) {
		ps.Spec.ServiceAccountName = kServiceAccount
	}
}

func WithCloudSchedulerSourceData(data string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Spec.Data = data
	}
}

func WithCloudSchedulerSourceDeletionTimestamp(s *v1.CloudSchedulerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithInitCloudSchedulerSourceConditions initializes the CloudSchedulerSources's conditions.
func WithInitCloudSchedulerSourceConditions(s *v1.CloudSchedulerSource) {
	s.Status.InitializeConditions()
}

func WithCloudSchedulerSourceWorkloadIdentityFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicFailed marks the condition that the
// status of topic is False.
func WithCloudSchedulerSourceTopicFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkTopicFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicUnknown marks the condition that the
// status of topic is Unknown.
func WithCloudSchedulerSourceTopicUnknown(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkTopicUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicNotReady marks the condition that the
// topic is not ready.
func WithCloudSchedulerSourceTopicReady(topicID, projectID string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkTopicReady(s.ConditionSet())
		s.Status.TopicID = topicID
		s.Status.ProjectID = projectID
	}
}

// WithCloudAuditLogsSourceTopicDeleted is a wrapper to indicate that the
// topic is deleted. Inside the function, we still mark the status of topic to be ready,
// as the status of topic is unchanged if the deletion is successful. We do not set the
// topicID and projectID because they are set to empty when deleting the topic.
func WithCloudSchedulerSourceTopicDeleted(s *v1.CloudSchedulerSource) {
	s.Status.MarkTopicReady(s.ConditionSet())
}

// WithCloudSchedulerSourcePullSubscriptionFailed marks the condition that the
// topic is False.
func WithCloudSchedulerSourcePullSubscriptionFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourcePullSubscriptionUnknown marks the condition that the
// topic is Unknown.
func WithCloudSchedulerSourcePullSubscriptionUnknown(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourcePullSubscriptionReady marks the condition that the
// topic is ready.
func WithCloudSchedulerSourcePullSubscriptionReady(s *v1.CloudSchedulerSource) {
	s.Status.MarkPullSubscriptionReady(s.ConditionSet())
}

// WithCloudSchedulerSourcePullSubscriptionDeleted is a wrapper to indicate that the
// PullSubscription is deleted. Inside the function, we still mark the status of
// PullSubscription to be ready, as the status of PullSubscription is unchanged
// if the deletion is successful.
func WithCloudSchedulerSourcePullSubscriptionDeleted(s *v1.CloudSchedulerSource) {
	s.Status.MarkPullSubscriptionReady(s.ConditionSet())
}

// WithCloudSchedulerSourceJobNotReady marks the condition that the
// CloudSchedulerSource Job is not ready.
func WithCloudSchedulerSourceJobNotReady(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkJobNotReady(reason, message)
	}
}

// WithCloudSchedulerSourceJobUnknown marks the condition that the
// CloudSchedulerSource Job is unknown.
func WithCloudSchedulerSourceJobUnknown(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkJobUnknown(reason, message)
	}
}

// WithCloudSchedulerSourceJobReady marks the condition that the
// CloudSchedulerSource Job is ready and sets Status.JobName to jobName.
func WithCloudSchedulerSourceJobReady(jobName string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkJobReady(jobName)
	}
}

// WithCloudSchedulerSourceJobDeleted is a wrapper to indicate that the
// job is deleted. Inside the function, we still mark the status of job to be ready,
// as the status of job is unchanged if the deletion is successful.
func WithCloudSchedulerSourceJobDeleted(jobName string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.MarkJobReady(jobName)
	}
}

// WithCloudSchedulerSourceSinkURI sets the status for sink URI
func WithCloudSchedulerSourceSinkURI(url *apis.URL) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.SinkURI = url
	}
}

func WithCloudSchedulerSourceSubscriptionID(subscriptionID string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.SubscriptionID = subscriptionID
	}
}

// WithCloudSchedulerSourceJobName sets the status for job Name
func WithCloudSchedulerSourceJobName(jobName string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Status.JobName = jobName
	}
}

func WithCloudSchedulerSourceFinalizers(finalizers ...string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.Finalizers = finalizers
	}
}

func WithCloudSchedulerSourceAnnotations(Annotations map[string]string) CloudSchedulerSourceOption {
	return func(s *v1.CloudSchedulerSource) {
		s.ObjectMeta.Annotations = Annotations
	}
}

func WithCloudSchedulerSourceSetDefaults(s *v1.CloudSchedulerSource) {
	s.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
}
