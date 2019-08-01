/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1alpha1"
)

// GetCondition returns the condition currently associated with the given type,
// or nil.
func (ts *TopicStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return topicCondSet.Manage(ts).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ts *TopicStatus) IsReady() bool {
	return topicCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TopicStatus) InitializeConditions() {
	topicCondSet.Manage(ts).InitializeConditions()
}

// TODO: Use the new beta duck types.
func (ts *TopicStatus) SetAddress(url *apis.URL) {
	if ts.Address == nil {
		ts.Address = &v1alpha1.Addressable{}
	}
	if url != nil {
		ts.Address.Hostname = url.Host
		ts.Address.URL = url
		topicCondSet.Manage(ts).MarkTrue(TopicConditionAddressable)
	} else {
		ts.Address.Hostname = ""
		ts.Address.URL = nil
		topicCondSet.Manage(ts).MarkFalse(TopicConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}

func (ts *TopicStatus) PropagatePublisherStatus(ready *apis.Condition) {
	switch {
	case ready == nil:
		ts.MarkDeploying("PublisherStatus", "Publisher has no Ready type status.")
	case ready.IsTrue():
		ts.MarkDeployed()
	case ready.IsFalse():
		ts.MarkNotDeployed(ready.Reason, ready.Message)
	default:
		ts.MarkDeploying(ready.Reason, ready.Message)
	}
}

// MarkDeployed sets the condition that the publisher has been deployed.
func (ts *TopicStatus) MarkDeployed() {
	topicCondSet.Manage(ts).MarkTrue(TopicConditionPublisherReady)
}

// MarkDeploying sets the condition that the publisher is deploying.
func (ts *TopicStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkUnknown(TopicConditionPublisherReady, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the publisher has not been deployed.
func (ts *TopicStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkFalse(TopicConditionPublisherReady, reason, messageFormat, messageA...)
}

func (ts *TopicStatus) MarkPublisherNotOwned(messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkFalse(TopicConditionPublisherReady, "NotOwned", messageFormat, messageA...)
}

// MarkTopicReady sets the condition that the topic has been created.
func (ts *TopicStatus) MarkTopicReady() {
	topicCondSet.Manage(ts).MarkTrue(TopicConditionTopicExists)
}

// MarkTopicOperating sets the condition that the topic is currently involved in an operation.
func (ts *TopicStatus) MarkTopicOperating(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkUnknown(TopicConditionTopicExists, reason, messageFormat, messageA...)
}

// MarkNoTopic sets the condition that signals there is not a topic for this
// Topic. This could be because of an error or the Topic is being deleted.
func (ts *TopicStatus) MarkNoTopic(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkFalse(TopicConditionTopicExists, reason, messageFormat, messageA...)
}
