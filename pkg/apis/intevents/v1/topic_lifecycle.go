/*
 * Copyright 2020 The Google LLC.
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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
)

// GetCondition returns the condition currently associated with the given type,
// or nil.
func (ts *TopicStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return topicCondSet.Manage(ts).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition
func (ts *TopicStatus) GetTopLevelCondition() *apis.Condition {
	return topicCondSet.Manage(ts).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (ts *TopicStatus) IsReady() bool {
	return topicCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TopicStatus) InitializeConditions() {
	topicCondSet.Manage(ts).InitializeConditions()
}

func (ts *TopicStatus) SetAddress(url *apis.URL) {
	if ts.Address == nil {
		ts.Address = &duckv1.Addressable{}
	}
	if url != nil {
		ts.Address.URL = url
		topicCondSet.Manage(ts).MarkTrue(TopicConditionAddressable)
	} else {
		ts.Address.URL = nil
		topicCondSet.Manage(ts).MarkFalse(TopicConditionAddressable, "emptyUrl", "url is the empty string")
		// The TopicConditionAddressable is not included in the ready set as we don't want to create Publishers for Sources.
		// We therefore need to set the ConditionReady to false here.
		topicCondSet.Manage(ts).MarkFalse(apis.ConditionReady, "emptyUrl", "url is the empty string")

	}
}

func (ts *TopicStatus) PropagatePublisherStatus(ss *v1.ServiceStatus) {
	sc := ss.GetCondition(apis.ConditionReady)
	if sc == nil {
		ts.MarkPublisherNotConfigured()
		return
	}

	switch {
	case sc.Status == corev1.ConditionUnknown:
		ts.MarkPublisherUnknown(sc.Reason, sc.Message)
	case sc.Status == corev1.ConditionTrue:
		ts.SetAddress(ss.Address.URL)
		ts.MarkPublisherDeployed()
	case sc.Status == corev1.ConditionFalse:
		ts.MarkPublisherNotDeployed(sc.Reason, sc.Message)
	default:
		ts.MarkPublisherUnknown("TopicUnknown", "The status of Topic is invalid: %v", sc.Status)
	}
}

// MarkPublisherDeployed sets the condition that the publisher has been deployed.
func (ts *TopicStatus) MarkPublisherDeployed() {
	topicCondSet.Manage(ts).MarkTrue(TopicConditionPublisherReady)
}

// MarkPublisherUnknown sets the condition that the status of publisher is Unknown.
func (ts *TopicStatus) MarkPublisherUnknown(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkUnknown(TopicConditionPublisherReady, reason, messageFormat, messageA...)
	// The TopicConditionPublisherReady is not included in the ready set as we don't want to create Publishers for Sources.
	// We therefore need to set the ConditionReady to unknown here.
	topicCondSet.Manage(ts).MarkUnknown(apis.ConditionReady, reason, messageFormat, messageA...)
}

// MarkPublisherNotDeployed sets the condition that the publisher has not been deployed.
func (ts *TopicStatus) MarkPublisherNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkFalse(TopicConditionPublisherReady, reason, messageFormat, messageA...)
	// The TopicConditionPublisherReady is not included in the ready set as we don't want to create Publishers for Sources.
	// We therefore need to set the ConditionReady to false here.
	topicCondSet.Manage(ts).MarkFalse(apis.ConditionReady, reason, messageFormat, messageA...)
}

// MarkPublisherNotConfigured changes the PublisherReady condition to be unknown to reflect
// that the Publisher does not yet have a Status.
func (ts *TopicStatus) MarkPublisherNotConfigured() {
	topicCondSet.Manage(ts).MarkUnknown(TopicConditionPublisherReady, "PublisherNotConfigured", "Publisher has not yet been reconciled")
	// The TopicConditionPublisherReady is not included in the ready set as we don't want to create Publishers for Sources.
	// We therefore need to set the ConditionReady to unknown here.
	topicCondSet.Manage(ts).MarkUnknown(apis.ConditionReady, "PublisherNotConfigured", "Publisher has not yet been reconciled")
}

// MarkTopicReady sets the condition that the topic has been created.
func (ts *TopicStatus) MarkTopicReady() {
	topicCondSet.Manage(ts).MarkTrue(TopicConditionTopicExists)
}

// MarkNoTopic sets the condition that signals there is not a topic for this
// Topic. This could be because of an error or the Topic is being deleted.
func (ts *TopicStatus) MarkNoTopic(reason, messageFormat string, messageA ...interface{}) {
	topicCondSet.Manage(ts).MarkFalse(TopicConditionTopicExists, reason, messageFormat, messageA...)
}
