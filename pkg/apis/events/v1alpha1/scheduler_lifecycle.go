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

package v1alpha1

import (
	"knative.dev/pkg/apis"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *SchedulerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return schedulerCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *SchedulerStatus) IsReady() bool {
	return schedulerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *SchedulerStatus) InitializeConditions() {
	schedulerCondSet.Manage(s).InitializeConditions()
}

// MarkPullSubscriptionNotReady sets the condition that the underlying PullSubscription
// is not ready and why
func (s *SchedulerStatus) MarkPullSubscriptionNotReady(reason, messageFormat string, messageA ...interface{}) {
	schedulerCondSet.Manage(s).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is ready
func (s *SchedulerStatus) MarkPullSubscriptionReady() {
	schedulerCondSet.Manage(s).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

// MarkTopicNotReady sets the condition that the Topic was not created and why
func (s *SchedulerStatus) MarkTopicNotReady(reason, messageFormat string, messageA ...interface{}) {
	schedulerCondSet.Manage(s).MarkFalse(duckv1alpha1.TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the underlying Topic was created
// successfully and sets the Status.TopicID to the specified topic
// and Status.ProjectID to the specified project.
func (s *SchedulerStatus) MarkTopicReady(topicID, projectID string) {
	schedulerCondSet.Manage(s).MarkTrue(duckv1alpha1.TopicReady)
	s.TopicID = topicID
	s.ProjectID = projectID
}

// MarkJobNotReady sets the condition that the Scheduler Job has not been
// successfully created.
func (s *SchedulerStatus) MarkJobNotReady(reason, messageFormat string, messageA ...interface{}) {
	schedulerCondSet.Manage(s).MarkFalse(JobReady, reason, messageFormat, messageA...)
}

// MarkJobReady sets the condition for Scheduler Job as Read and sets the
// Status.JobName to jobName
func (s *SchedulerStatus) MarkJobReady(jobName string) {
	schedulerCondSet.Manage(s).MarkTrue(JobReady)
	s.JobName = jobName
}
