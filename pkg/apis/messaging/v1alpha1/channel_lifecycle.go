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
	"knative.dev/pkg/apis/duck/v1beta1"
)

// GetCondition returns the condition currently associated with the given type,
// or nil.
func (cs *ChannelStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return channelCondSet.Manage(cs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (cs *ChannelStatus) IsReady() bool {
	return channelCondSet.Manage(cs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (cs *ChannelStatus) InitializeConditions() {
	channelCondSet.Manage(cs).InitializeConditions()
}

// SetAddress updates the Addressable status of the channel and propagates a
// url status to the Addressable status condition based on url.
func (cs *ChannelStatus) SetAddress(url *apis.URL) {
	if cs.Address == nil {
		cs.Address = &v1beta1.Addressable{}
	}
	if url != nil {
		cs.Address.URL = url
		channelCondSet.Manage(cs).MarkTrue(ChannelConditionAddressable)
	} else {
		cs.Address.URL = nil
		channelCondSet.Manage(cs).MarkFalse(ChannelConditionAddressable, "emptyUrl", "url is empty")
	}
}

// MarkTopicReady sets the condition that the topic has been created and ready.
func (cs *ChannelStatus) MarkTopicReady() {
	channelCondSet.Manage(cs).MarkTrue(ChannelConditionTopicReady)
}

// MarkTopicOperating sets the condition that the topic is being created.
func (cs *ChannelStatus) MarkTopicOperating(reason, messageFormat string, messageA ...interface{}) {
	channelCondSet.Manage(cs).MarkUnknown(ChannelConditionTopicReady, reason, messageFormat, messageA...)
}

// MarkNoTopic sets the condition that signals there is not a topic for this
// Channel. This could be because of an error or the Channel is being deleted.
func (cs *ChannelStatus) MarkNoTopic(reason, messageFormat string, messageA ...interface{}) {
	channelCondSet.Manage(cs).MarkFalse(ChannelConditionTopicReady, reason, messageFormat, messageA...)
}

func (cs *ChannelStatus) MarkTopicNotOwned(messageFormat string, messageA ...interface{}) {
	channelCondSet.Manage(cs).MarkFalse(ChannelConditionTopicReady, "NotOwned", messageFormat, messageA...)
}
