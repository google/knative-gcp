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

package v1

import (
	"knative.dev/pkg/apis"
)

func (s *IdentityStatus) MarkWorkloadIdentityReady(cs *apis.ConditionSet) {
	cs.Manage(s).MarkTrue(IdentityConfigured)
}

func (s *IdentityStatus) MarkWorkloadIdentityFailed(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkFalse(IdentityConfigured, reason, messageFormat, messageA...)
	// Set ConditionReady to be false.
	// ConditionType IdentityConfigured is not included in apis.NewLivingConditionSet{}, so it is not counted for conditionReady.
	// This is because if Workload Identity is not enabled, IdentityConfigured will be unknown.
	// It will be counted for conditionReady only if it is failed.
	cs.Manage(s).MarkFalse(apis.ConditionReady, "WorkloadIdentityFailed", messageFormat, messageA...)
}

func (s *IdentityStatus) MarkWorkloadIdentityUnknown(cs *apis.ConditionSet, reason, messageFormat string, messageA ...interface{}) {
	cs.Manage(s).MarkUnknown(IdentityConfigured, reason, messageFormat, messageA...)
	// ConditionType IdentityConfigured is not included in apis.NewLivingConditionSet{}, so it is not counted for conditionReady.
	// Set ConditionReady to be Unknown if the initial status of ConditionReady is Ready.
	// If the initial status of ConditionReady is not Ready, we keep it as it.
	if c := cs.Manage(s).GetCondition(apis.ConditionReady); c.IsTrue() {
		cs.Manage(s).MarkUnknown(apis.ConditionReady, "WorkloadIdentityUnknown", messageFormat, messageA...)
	}
}
