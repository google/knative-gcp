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

package v1alpha1

import (
duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
corev1 "k8s.io/api/core/v1"
"knative.dev/pkg/apis"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *CloudBuildSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return buildCondSet.Manage(bs).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition.
func (bs *CloudBuildSourceStatus) GetTopLevelCondition() *apis.Condition {
	return buildCondSet.Manage(bs).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (bs *CloudBuildSourceStatus) IsReady() bool {
	return buildCondSet.Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *CloudBuildSourceStatus) InitializeConditions() {
	buildCondSet.Manage(bs).InitializeConditions()
}

// MarkPullSubscriptionFailed sets the condition that the underlying PullSubscription
// is False and why.
func (bs *CloudBuildSourceStatus) MarkPullSubscriptionFailed(reason, messageFormat string, messageA ...interface{}) {
	buildCondSet.Manage(bs).MarkFalse(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

// MarkPullSubscriptionNotConfigured changes the PullSubscriptionReady condition to be unknown to reflect
// that the PullSubscription does not yet have a Status.
func (bs *CloudBuildSourceStatus) MarkPullSubscriptionNotConfigured() {
	buildCondSet.Manage(bs).MarkUnknown(duckv1alpha1.PullSubscriptionReady, "PullSubscriptionNotConfigured", "PullSubscription has not yet been reconciled")
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is ready.
func (bs *CloudBuildSourceStatus) MarkPullSubscriptionReady() {
	buildCondSet.Manage(bs).MarkTrue(duckv1alpha1.PullSubscriptionReady)
}

// MarkPullSubscriptionReady sets the condition that the underlying PullSubscription is Unknown and why.
func (bs *CloudBuildSourceStatus) MarkPullSubscriptionUnknown(reason, messageFormat string, messageA ...interface{}) {
	buildCondSet.Manage(bs).MarkUnknown(duckv1alpha1.PullSubscriptionReady, reason, messageFormat, messageA...)
}

func (bs *CloudBuildSourceStatus) PropagatePullSubscriptionStatus(pss *v1alpha1.PullSubscriptionStatus) {
	psc := pss.GetTopLevelCondition()
	if psc == nil {
		bs.MarkPullSubscriptionNotConfigured()
		return
	}

	switch {
	case psc.Status == corev1.ConditionUnknown:
		bs.MarkPullSubscriptionUnknown(psc.Reason, psc.Message)
	case psc.Status == corev1.ConditionTrue:
		bs.MarkPullSubscriptionReady()
	case psc.Status == corev1.ConditionFalse:
		bs.MarkPullSubscriptionFailed(psc.Reason, psc.Message)
	default:
		bs.MarkPullSubscriptionUnknown("PullSubscriptionUnknown", "The status of PullSubscription is invalid: %v", psc.Status)
	}
}
