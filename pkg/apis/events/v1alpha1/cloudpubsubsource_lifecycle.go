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
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *CloudPubSubSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pubSubCondSet.Manage(ps).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition.
func (ps *CloudPubSubSourceStatus) GetTopLevelCondition() *apis.Condition {
	return pubSubCondSet.Manage(ps).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (ps *CloudPubSubSourceStatus) IsReady() bool {
	return pubSubCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *CloudPubSubSourceStatus) InitializeConditions() {
	pubSubCondSet.Manage(ps).InitializeConditions()
}
