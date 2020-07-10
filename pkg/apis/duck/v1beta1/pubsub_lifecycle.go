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

package v1beta1

import (
	"knative.dev/pkg/apis"
)

// IsReady returns true if the resource is ready overall.
func (ss *PubSubStatus) IsReady() bool {
	for _, c := range ss.Conditions {
		switch c.Type {
		// Look for the "happy" condition, which is the only condition that
		// we can reliably understand to be the overall state of the resource.
		case apis.ConditionReady, apis.ConditionSucceeded:
			return c.IsTrue()
		}
	}
	return false
}

// MarkTopicFailed sets the condition that the PubSub Topic is False and why.
func (s *PubSubStatus) MarkTopicFailed(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkFalse(TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicUnknown sets the condition that the PubSub Topic is Unknown and why.
func (s *PubSubStatus) MarkTopicUnknown(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkUnknown(TopicReady, reason, messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the PubSub Topic is ready.
func (s *PubSubStatus) MarkTopicReady(cs *apis.ConditionSet) {
	cs.Manage(s).MarkTrue(TopicReady)
}

// MarkTopicNotConfigured changes the TopicReady condition to be unknown to reflect
// that the Topic does not yet have a Status.
func (s *PubSubStatus) MarkTopicNotConfigured(cs *apis.ConditionSet) {
	cs.Manage(s).MarkUnknown(TopicReady, "TopicNotConfigured", "Topic has not yet been reconciled")
}

// MarkPullSubscriptionFailed sets the condition that the PubSub PullSubscription is
// False and why.
func (s *PubSubStatus) MarkPullSubscriptionFailed(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkFalse(PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionUnknown sets the condition that the PubSub PullSubscription is Unknown.
func (s *PubSubStatus) MarkPullSubscriptionUnknown(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkUnknown(PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the PubSub PullSubscription is ready.
func (s *PubSubStatus) MarkPullSubscriptionReady(cs *apis.ConditionSet) {
	cs.Manage(s).MarkTrue(PullSubscriptionReady)
}

// MarkPullSubscriptionNotConfigured changes the PullSubscriptionReady condition to be unknown to reflect
// that the PullSubscription does not yet have a Status.
func (s *PubSubStatus) MarkPullSubscriptionNotConfigured(cs *apis.ConditionSet) {
	cs.Manage(s).MarkUnknown(PullSubscriptionReady, "PullSubscriptionNotConfigured", "PullSubscription has not yet been reconciled")
}
