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
	corev1 "k8s.io/api/core/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var triggerCondSet = apis.NewLivingConditionSet(
	eventingv1beta1.TriggerConditionBroker,
	eventingv1beta1.TriggerConditionDependency,
	eventingv1beta1.TriggerConditionSubscriberResolved,
	TriggerConditionTopic,
	TriggerConditionSubscription,
)

const (
	TriggerConditionTopic        apis.ConditionType = "TopicReady"
	TriggerConditionSubscription apis.ConditionType = "SubscriptionReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ts *TriggerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return triggerCondSet.Manage(ts).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (ts *TriggerStatus) GetTopLevelCondition() *apis.Condition {
	return triggerCondSet.Manage(ts).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (ts *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(ts).InitializeConditions()
}

func (ts *TriggerStatus) PropagateBrokerStatus(bs *BrokerStatus) {
	bc := bs.GetTopLevelCondition()
	if bc == nil {
		ts.MarkBrokerNotConfigured()
		return
	}

	switch {
	case bc.Status == corev1.ConditionUnknown:
		ts.MarkBrokerUnknown("Broker/"+bc.Reason, bc.Message)
	case bc.Status == corev1.ConditionTrue:
		triggerCondSet.Manage(ts).MarkTrue(eventingv1beta1.TriggerConditionBroker)
	case bc.Status == corev1.ConditionFalse:
		ts.MarkBrokerFailed("Broker/"+bc.Reason, bc.Message)
	default:
		ts.MarkBrokerUnknown("BrokerUnknown", "The status of Broker is invalid: %v", bc.Status)
	}
}

func (ts *TriggerStatus) MarkBrokerFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(eventingv1beta1.TriggerConditionBroker, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkBrokerUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(eventingv1beta1.TriggerConditionBroker, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkBrokerNotConfigured() {
	triggerCondSet.Manage(ts).MarkUnknown(eventingv1beta1.TriggerConditionBroker,
		"BrokerNotConfigured", "Broker has not yet been reconciled.")
}

func (bs *TriggerStatus) MarkTopicFailed(reason, format string, args ...interface{}) {
	triggerCondSet.Manage(bs).MarkFalse(TriggerConditionTopic, reason, format, args...)
}

func (bs *TriggerStatus) MarkTopicUnknown(reason, format string, args ...interface{}) {
	triggerCondSet.Manage(bs).MarkUnknown(TriggerConditionTopic, reason, format, args...)
}

func (bs *TriggerStatus) MarkTopicReady() {
	triggerCondSet.Manage(bs).MarkTrue(TriggerConditionTopic)
}

func (bs *TriggerStatus) MarkSubscriptionFailed(reason, format string, args ...interface{}) {
	triggerCondSet.Manage(bs).MarkFalse(TriggerConditionSubscription, reason, format, args...)
}

func (bs *TriggerStatus) MarkSubscriptionUnknown(reason, format string, args ...interface{}) {
	triggerCondSet.Manage(bs).MarkUnknown(TriggerConditionSubscription, reason, format, args...)
}

func (bs *TriggerStatus) MarkSubscriptionReady() {
	triggerCondSet.Manage(bs).MarkTrue(TriggerConditionSubscription)
}

func (ts *TriggerStatus) MarkSubscriberResolvedSucceeded() {
	triggerCondSet.Manage(ts).MarkTrue(eventingv1beta1.TriggerConditionSubscriberResolved)
}

func (ts *TriggerStatus) MarkSubscriberResolvedFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(eventingv1beta1.TriggerConditionSubscriberResolved, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkSubscriberResolvedUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(eventingv1beta1.TriggerConditionSubscriberResolved, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkDependencySucceeded() {
	triggerCondSet.Manage(ts).MarkTrue(eventingv1beta1.TriggerConditionDependency)
}

func (ts *TriggerStatus) MarkDependencyFailed(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(eventingv1beta1.TriggerConditionDependency, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkDependencyUnknown(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkUnknown(eventingv1beta1.TriggerConditionDependency, reason, messageFormat, messageA...)
}

func (ts *TriggerStatus) MarkDependencyNotConfigured() {
	triggerCondSet.Manage(ts).MarkUnknown(eventingv1beta1.TriggerConditionDependency,
		"DependencyNotConfigured", "Dependency has not yet been reconciled.")
}

func (ts *TriggerStatus) PropagateDependencyStatus(src *duckv1.Source) {
	sc := src.Status.GetCondition(apis.ConditionReady)
	if sc == nil {
		ts.MarkDependencyNotConfigured()
		return
	}

	switch {
	case sc.Status == corev1.ConditionUnknown:
		ts.MarkDependencyUnknown(sc.Reason, sc.Message)
	case sc.Status == corev1.ConditionTrue:
		ts.MarkDependencySucceeded()
	case sc.Status == corev1.ConditionFalse:
		ts.MarkDependencyFailed(sc.Reason, sc.Message)
	default:
		ts.MarkDependencyUnknown("DependencyUnknown", "The status of Dependency is invalid: %v", sc.Status)
	}
}
