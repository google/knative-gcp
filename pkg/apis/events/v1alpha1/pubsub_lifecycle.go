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
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *PubSubStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pubSubCondSet.Manage(ps).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition.
func (ps *PubSubStatus) GetTopLevelCondition() *apis.Condition {
	return pubSubCondSet.Manage(ps).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (ps *PubSubStatus) IsReady() bool {
	return pubSubCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *PubSubStatus) InitializeConditions() {
	pubSubCondSet.Manage(ps).InitializeConditions()
}

// MarkPullSubscriptionFailed sets the condition that the underlying PullSubscription
// source is False and why.
func (ps *PubSubStatus) MarkPullSubscriptionFailed(reason, messageFormat string, messageA ...interface{}) {
	pubSubCondSet.Manage(ps).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionNotConfigured changes the PullSubscriptionReady condition to be unknown to reflect
// that the PullSubscription does not yet have a Status.
func (ps *PubSubStatus) MarkPullSubscriptionNotConfigured() {
	pubSubCondSet.Manage(ps).MarkUnknown(duckv1alpha1.PullSubscriptionReady, "PullSubscriptionNotConfigured", "PullSubscription has not yet been reconciled")
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is ready.
func (ps *PubSubStatus) MarkPullSubscriptionReady() {
	pubSubCondSet.Manage(ps).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is Unknown and why.
func (ps *PubSubStatus) MarkPullSubscriptionUnknown(reason, messageFormat string, messageA ...interface{}) {
	pubSubCondSet.Manage(ps).MarkUnknown(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

func (ps *PubSubStatus) PropagatePullSubscriptionStatus(ready *apis.Condition) {
	if ready == nil {
		ps.MarkPullSubscriptionNotConfigured()
		return
	}

	switch {
	case ready.Status == corev1.ConditionUnknown:
		ps.MarkPullSubscriptionUnknown(ready.Reason, ready.Message)
	case ready.Status == corev1.ConditionTrue:
		ps.MarkPullSubscriptionReady()
	case ready.Status == corev1.ConditionFalse:
		ps.MarkPullSubscriptionFailed(ready.Reason, ready.Message)
	default:
		ps.MarkPullSubscriptionUnknown("PullSubscriptionUnknown", "The status of PullSubscription is invalid: %v", ready.Status)
	}
}
