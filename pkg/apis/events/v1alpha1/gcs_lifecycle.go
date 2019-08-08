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
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GCSStatus) GetCondition(t duckv1beta1.ConditionType) *duckv1beta1.Condition {
	return gcsSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *GCSStatus) IsReady() bool {
	return gcsSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *GCSStatus) InitializeConditions() {
	gcsSourceCondSet.Manage(s).InitializeConditions()
}

// MarkPubSubNotSourceReady sets the condition that the underlying PubSub source is not ready and why
func (s *GCSStatus) MarkPubSubSourceNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(PubSubSourceReady, reason, messageFormat, messageA...)
}

// MarkPubSubSourceReady sets the condition that the underlying PubSub source is ready
func (s *GCSStatus) MarkPubSubSourceReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(PubSubSourceReady)
}

// MarkPubSubTopicNotReady sets the condition that the PubSub topic was not created and why
func (s *GCSStatus) MarkPubSubTopicNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(PubSubTopicReady, reason, messageFormat, messageA...)
}

// MarkPubSubTopicReady sets the condition that the underlying PubSub topic was created successfully
func (s *GCSStatus) MarkPubSubTopicReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(PubSubTopicReady)
}

// MarkGCSNotReady sets the condition that the GCS has been configured to send Notifications
func (s *GCSStatus) MarkGCSNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(GCSReady, reason, messageFormat, messageA...)
}

func (s *GCSStatus) MarkGCSReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(GCSReady)
}
