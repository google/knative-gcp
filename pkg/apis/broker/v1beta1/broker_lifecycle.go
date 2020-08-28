/*
Copyright 2020 Google LLC

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
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

var brokerCondSet = apis.NewLivingConditionSet(
	eventingv1beta1.BrokerConditionAddressable,
	BrokerConditionBrokerCell,
	BrokerConditionTopic,
	BrokerConditionSubscription,
)

const (
	// BrokerConditionBrokerCell reports the availability of the Broker's BrokerCell.
	BrokerConditionBrokerCell apis.ConditionType = "BrokerCellReady"
	// BrokerConditionTopic reports the status of the Broker's PubSub topic.
	// THis condition is specific to the Google Cloud Broker.
	BrokerConditionTopic apis.ConditionType = "TopicReady"
	// BrokerConditionSubscription reports the status of the Broker's PubSub
	// subscription. This condition is specific to the Google Cloud Broker.
	BrokerConditionSubscription apis.ConditionType = "SubscriptionReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return brokerCondSet.Manage(bs).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (bs *BrokerStatus) GetTopLevelCondition() *apis.Condition {
	return brokerCondSet.Manage(bs).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (bs *BrokerStatus) IsReady() bool {
	return brokerCondSet.Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	brokerCondSet.Manage(bs).InitializeConditions()
}

// SetAddress makes this Broker addressable by setting the hostname. It also
// sets the BrokerConditionAddressable to true.
func (bs *BrokerStatus) SetAddress(url *apis.URL) {
	bs.Address.URL = url
	if url != nil {
		brokerCondSet.Manage(bs).MarkTrue(eventingv1beta1.BrokerConditionAddressable)
	} else {
		brokerCondSet.Manage(bs).MarkFalse(eventingv1beta1.BrokerConditionAddressable, "emptyURL", "URL is empty")
	}
}

func (bs *BrokerStatus) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkUnknown(BrokerConditionBrokerCell, reason, format, args...)
}

func (bs *BrokerStatus) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionBrokerCell, reason, format, args...)
}

func (bs *BrokerStatus) MarkBrokerCellReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionBrokerCell)
}

func (bs *BrokerStatus) MarkTopicFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionTopic, reason, format, args...)
}

func (bs *BrokerStatus) MarkTopicUnknown(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkUnknown(BrokerConditionTopic, reason, format, args...)
}

func (bs *BrokerStatus) MarkTopicReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionTopic)
}

func (bs *BrokerStatus) MarkSubscriptionFailed(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionSubscription, reason, format, args...)
}

func (bs *BrokerStatus) MarkSubscriptionUnknown(reason, format string, args ...interface{}) {
	brokerCondSet.Manage(bs).MarkUnknown(BrokerConditionSubscription, reason, format, args...)
}

func (bs *BrokerStatus) MarkSubscriptionReady() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionSubscription)
}
