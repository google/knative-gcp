/*
Copyright 2019 Google LLC.

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
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *AuditLogsSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return auditLogsSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *AuditLogsSourceStatus) IsReady() bool {
	return auditLogsSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *AuditLogsSourceStatus) InitializeConditions() {
	auditLogsSourceCondSet.Manage(s).InitializeConditions()
}

// MarkPullSubscriptionNotReady sets the condition that the underlying PullSubscription
// source is not ready and why.
func (s *AuditLogsSourceStatus) MarkPullSubscriptionNotReady(reason, messageFormat string, messageA ...interface{}) {
	auditLogsSourceCondSet.Manage(s).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the underlying PubSub source is ready.
func (s *AuditLogsSourceStatus) MarkPullSubscriptionReady() {
	auditLogsSourceCondSet.Manage(s).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

// MarkTopicNotReady sets the condition that the PubSub topic was not created and why.
func (s *AuditLogsSourceStatus) MarkTopicNotReady(reason, messageFormat string, messageA ...interface{}) {
	auditLogsSourceCondSet.Manage(s).MarkFalse(duckv1alpha1.TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the underlying PubSub topic was created successfully.
func (s *AuditLogsSourceStatus) MarkTopicReady() {
	auditLogsSourceCondSet.Manage(s).MarkTrue(duckv1alpha1.TopicReady)
}

// MarkSinkNotReady sets the condition that a AuditLogsSource pubsub sink
// has not been configured and why.
func (s *AuditLogsSourceStatus) MarkSinkNotReady(reason, messageFormat string, messageA ...interface{}) {
	auditLogsSourceCondSet.Manage(s).MarkFalse(SinkReady, reason, messageFormat, messageA...)
}

func (s *AuditLogsSourceStatus) MarkSinkReady() {
	auditLogsSourceCondSet.Manage(s).MarkTrue(SinkReady)
}
