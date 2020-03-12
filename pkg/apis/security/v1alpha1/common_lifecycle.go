/*
Copyright 2020 Google LLC.

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

import "knative.dev/pkg/apis"

var policybindingCondSet = apis.NewLivingConditionSet(PolicyBindingClassCompatible)

const (
	// PolicyBindingConditionReady has status True when the binding is active.
	PolicyBindingConditionReady = apis.ConditionReady

	// PolicyBindingClassCompatible has status True if the binding spec is
	// compatible with the specified binding class.
	PolicyBindingClassCompatible apis.ConditionType = "BindingClassCompatible"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (pbs *PolicyBindingStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return policybindingCondSet.Manage(pbs).GetCondition(t)
}

// GetTopLevelCondition returns the top level Condition.
func (pbs *PolicyBindingStatus) GetTopLevelCondition() *apis.Condition {
	return policybindingCondSet.Manage(pbs).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (pbs *PolicyBindingStatus) IsReady() bool {
	return policybindingCondSet.Manage(pbs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (pbs *PolicyBindingStatus) InitializeConditions() {
	policybindingCondSet.Manage(pbs).InitializeConditions()
}

// SetObservedGeneration implements psbinding.BindableStatus
func (pbs *PolicyBindingStatus) SetObservedGeneration(gen int64) {
	pbs.ObservedGeneration = gen
}

// MarkBindingUnavailable marks the policy binding's Ready condition to False with
// the provided reason and message.
// This implements psbinding.BindableStatus
func (pbs *PolicyBindingStatus) MarkBindingUnavailable(reason, message string) {
	policybindingCondSet.Manage(pbs).MarkFalse(PolicyBindingConditionReady, reason, message)
}

// MarkBindingFailure marks the policy binding's Ready condition to False with
// the provided reason and message.
// This function is the same as MarkBindingUnavailable with a more friendly function signature.
func (pbs *PolicyBindingStatus) MarkBindingFailure(reason, messageFormat string, messageA ...interface{}) {
	policybindingCondSet.Manage(pbs).MarkFalse(PolicyBindingConditionReady, reason, messageFormat, messageA...)
}

// MarkBindingAvailable marks the policy binding's Ready condition to True.
// This implements psbinding.BindableStatus.
func (pbs *PolicyBindingStatus) MarkBindingAvailable() {
	policybindingCondSet.Manage(pbs).MarkTrue(PolicyBindingConditionReady)
}

// MarkBindingClassCompatible marks the policy binding's class
// compatible status to True.
func (pbs *PolicyBindingStatus) MarkBindingClassCompatible() {
	policybindingCondSet.Manage(pbs).MarkTrue(PolicyBindingClassCompatible)
}

// MarkBindingClassIncompatible marks the policy binding's class
// compatible status to False.
func (pbs *PolicyBindingStatus) MarkBindingClassIncompatible(reason, messageFormat string, messageA ...interface{}) {
	policybindingCondSet.Manage(pbs).MarkFalse(PolicyBindingClassCompatible, reason, messageFormat, messageA...)
}
