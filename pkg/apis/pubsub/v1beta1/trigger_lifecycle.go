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

package v1beta1

import "knative.dev/pkg/apis"

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *TriggerStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return triggerCondSet.Manage(s).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition.
func (s *TriggerStatus) GetTopLevelCondition() *apis.Condition {
	return triggerCondSet.Manage(s).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (s *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(s).InitializeConditions()
}

// MarkTriggerNotReady sets the condition that EventFlow has not been configured
// to send Triggers and why.
func (s *TriggerStatus) MarkTriggerNotReady(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(s).MarkFalse(TriggerReady, reason, messageFormat, messageA...)
}

func (s *TriggerStatus) MarkTriggerReady(triggerID string) {
	s.TriggerID = triggerID
	triggerCondSet.Manage(s).MarkTrue(TriggerReady)
}
