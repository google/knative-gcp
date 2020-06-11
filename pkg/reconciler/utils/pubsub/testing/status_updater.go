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

package testing

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

type StatusUpdater struct {
	TopicCondition apis.Condition
	SubCondition   apis.Condition
}

func (su *StatusUpdater) MarkTopicFailed(reason, format string, args ...interface{}) {
	su.TopicCondition = apis.Condition{
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}
func (su *StatusUpdater) MarkTopicUnknown(reason, format string, args ...interface{}) {
	su.TopicCondition = apis.Condition{
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}
func (su *StatusUpdater) MarkTopicReady() {
	su.TopicCondition = apis.Condition{
		Status: corev1.ConditionTrue,
	}
}
func (su *StatusUpdater) MarkSubscriptionFailed(reason, format string, args ...interface{}) {
	su.SubCondition = apis.Condition{
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}
func (su *StatusUpdater) MarkSubscriptionUnknown(reason, format string, args ...interface{}) {
	su.SubCondition = apis.Condition{
		Status:  corev1.ConditionUnknown,
		Reason:  reason,
		Message: fmt.Sprintf(format, args...),
	}
}
func (su *StatusUpdater) MarkSubscriptionReady() {
	su.SubCondition = apis.Condition{
		Status: corev1.ConditionTrue,
	}
}
