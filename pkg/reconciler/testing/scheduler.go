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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// CloudSchedulerSourceOption enables further configuration of a CloudSchedulerSource.
type CloudSchedulerSourceOption func(*v1alpha1.CloudSchedulerSource)

// NewCloudSchedulerSource creates a CloudSchedulerSource with CloudSchedulerSourceOptions
func NewCloudSchedulerSource(name, namespace string, so ...CloudSchedulerSourceOption) *v1alpha1.CloudSchedulerSource {
	s := &v1alpha1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-scheduler-uid",
		},
	}
	for _, opt := range so {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

func WithCloudSchedulerSourceSink(gvk metav1.GroupVersionKind, name string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Spec.Sink = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
			},
		}
	}
}

func WithCloudSchedulerSourceLocation(location string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Spec.Location = location
	}
}

func WithCloudSchedulerSourceProject(project string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Spec.Project = project
	}
}

func WithCloudSchedulerSourceSchedule(schedule string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Spec.Schedule = schedule
	}
}

func WithCloudSchedulerSourceGCPServiceAccount(gServiceAccount string) CloudSchedulerSourceOption {
	return func(ps *v1alpha1.CloudSchedulerSource) {
		ps.Spec.GoogleServiceAccount = gServiceAccount
	}
}

func WithCloudSchedulerSourceData(data string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Spec.Data = data
	}
}

func WithCloudSchedulerSourceDeletionTimestamp(s *v1alpha1.CloudSchedulerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	s.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithInitCloudSchedulerSourceConditions initializes the CloudSchedulerSources's conditions.
func WithInitCloudSchedulerSourceConditions(s *v1alpha1.CloudSchedulerSource) {
	s.Status.InitializeConditions()
}

// WithCloudSchedulerSourceServiceAccountName will give status.ServiceAccountName a k8s service account name, which is related on Workload Identity's Google service account.
func WithCloudSchedulerSourceServiceAccountName(name string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.ServiceAccountName = name
	}
}

func WithCloudSchedulerSourceWorkloadIdentityFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkWorkloadIdentityFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicFailed marks the condition that the
// status of topic is False.
func WithCloudSchedulerSourceTopicFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkTopicFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicUnknown marks the condition that the
// status of topic is Unknown.
func WithCloudSchedulerSourceTopicUnknown(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkTopicUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourceTopicNotReady marks the condition that the
// topic is not ready.
func WithCloudSchedulerSourceTopicReady(topicID, projectID string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkTopicReady(s.ConditionSet())
		s.Status.TopicID = topicID
		s.Status.ProjectID = projectID
	}
}

// WithCloudSchedulerSourcePullSubscriptionFailed marks the condition that the
// topic is False.
func WithCloudSchedulerSourcePullSubscriptionFailed(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourcePullSubscriptionUnknown marks the condition that the
// topic is Unknown.
func WithCloudSchedulerSourcePullSubscriptionUnknown(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), reason, message)
	}
}

// WithCloudSchedulerSourcePullSubscriptionReady marks the condition that the
// topic is ready.
func WithCloudSchedulerSourcePullSubscriptionReady() CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkPullSubscriptionReady(s.ConditionSet())
	}
}

// WithCloudSchedulerSourceJobNotReady marks the condition that the
// CloudSchedulerSource Job is not ready.
func WithCloudSchedulerSourceJobNotReady(reason, message string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkJobNotReady(reason, message)
	}
}

// WithCloudSchedulerSourceJobReady marks the condition that the
// CloudSchedulerSource Job is ready and sets Status.JobName to jobName.
func WithCloudSchedulerSourceJobReady(jobName string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.MarkJobReady(jobName)
	}
}

// WithCloudSchedulerSourceSinkURI sets the status for sink URI
func WithCloudSchedulerSourceSinkURI(url *apis.URL) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.SinkURI = url
	}
}

// WithCloudSchedulerSourceJobName sets the status for job Name
func WithCloudSchedulerSourceJobName(jobName string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Status.JobName = jobName
	}
}

func WithCloudSchedulerSourceFinalizers(finalizers ...string) CloudSchedulerSourceOption {
	return func(s *v1alpha1.CloudSchedulerSource) {
		s.Finalizers = finalizers
	}
}
