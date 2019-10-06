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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *PubSubStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return PubSubCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *PubSubStatus) IsReady() bool {
	return PubSubCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *PubSubStatus) InitializeConditions() {
	PubSubCondSet.Manage(ps).InitializeConditions()
}

// MarkPullSubscriptionNotReady sets the condition that the underlying PullSubscription
// source is not ready and why.
func (ps *PubSubStatus) MarkPullSubscriptionNotReady(reason, messageFormat string, messageA ...interface{}) {
	PubSubCondSet.Manage(ps).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is ready.
func (ps *PubSubStatus) MarkPullSubscriptionReady() {
	PubSubCondSet.Manage(ps).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

func (ps *PubSubStatus) PropagatePullSubscriptionStatus(ready *apis.Condition) {
	switch {
	case ready == nil:
		ps.MarkPullSubscriptionNotReady("PullSubscriptionNotReady", "PullSubscription has no Ready type status")
	case ready.IsTrue():
		ps.MarkPullSubscriptionReady()
	case ready.IsFalse():
		ps.MarkPullSubscriptionNotReady(ready.Reason, ready.Message)
	}
}
