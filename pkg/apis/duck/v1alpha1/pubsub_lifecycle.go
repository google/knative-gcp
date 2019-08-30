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

// MarkTopicNotReady sets the condition that the PubSub Topic is not ready and why.
func (s *PubSubStatus) MarkTopicNotReady(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkFalse(TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the PubSub Topic is ready.
func (s *PubSubStatus) MarkTopicReady(cs *apis.ConditionSet) {
	cs.Manage(s).MarkTrue(TopicReady)
}

// MarkPullSubscriptionNotReady sets the condition that the PubSub PullSUbscription is
// not ready and why.
func (s *PubSubStatus) MarkPullSubscriptionNotReady(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkFalse(PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the PubSub PullSubscription is ready.
func (s *PubSubStatus) MarkPullSubscriptionReady(cs *apis.ConditionSet) {
	cs.Manage(s).MarkTrue(PullSubscriptionReady)
}
