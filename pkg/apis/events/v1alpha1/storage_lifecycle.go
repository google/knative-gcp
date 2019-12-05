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
func (s *StorageStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return storageCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *StorageStatus) IsReady() bool {
	return storageCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *StorageStatus) InitializeConditions() {
	storageCondSet.Manage(s).InitializeConditions()
}

// MarkPullSubscriptionNotReady sets the condition that the underlying PullSubscription
// source is not ready and why.
func (s *StorageStatus) MarkPullSubscriptionNotReady(reason, messageFormat string, messageA ...interface{}) {
	storageCondSet.Manage(s).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the underlying PubSub source is ready.
func (s *StorageStatus) MarkPullSubscriptionReady() {
	storageCondSet.Manage(s).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

// MarkTopicNotReady sets the condition that the PubSub topic was not created and why.
func (s *StorageStatus) MarkTopicNotReady(reason, messageFormat string, messageA ...interface{}) {
	storageCondSet.Manage(s).MarkFalse(duckv1alpha1.TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the underlying PubSub topic was created successfully.
func (s *StorageStatus) MarkTopicReady() {
	storageCondSet.Manage(s).MarkTrue(duckv1alpha1.TopicReady)
}

// MarkNotificationNotReady sets the condition that the GCS has not been configured
// to send Notifications and why.
func (s *StorageStatus) MarkNotificationNotReady(reason, messageFormat string, messageA ...interface{}) {
	storageCondSet.Manage(s).MarkFalse(NotificationReady, reason, messageFormat, messageA...)
}

func (s *StorageStatus) MarkNotificationReady(notificationID string) {
	s.NotificationID = notificationID
	storageCondSet.Manage(s).MarkTrue(NotificationReady)
}
