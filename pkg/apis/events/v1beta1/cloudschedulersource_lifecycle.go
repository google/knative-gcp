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

package v1beta1

import (
	"knative.dev/pkg/apis"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CloudSchedulerSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return schedulerCondSet.Manage(s).GetCondition(t)
}

// GetTopLevelCondition returns the top level condition.
func (s *CloudSchedulerSourceStatus) GetTopLevelCondition() *apis.Condition {
	return schedulerCondSet.Manage(s).GetTopLevelCondition()
}

// IsReady returns true if the resource is ready overall.
func (s *CloudSchedulerSourceStatus) IsReady() bool {
	return schedulerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CloudSchedulerSourceStatus) InitializeConditions() {
	schedulerCondSet.Manage(s).InitializeConditions()
}

// MarkJobNotReady sets the condition that the CloudSchedulerSource Job has not been
// successfully created.
func (s *CloudSchedulerSourceStatus) MarkJobNotReady(reason, messageFormat string, messageA ...interface{}) {
	schedulerCondSet.Manage(s).MarkFalse(JobReady, reason, messageFormat, messageA...)
}

// MarkJobReady sets the condition for CloudSchedulerSource Job as Read and sets the
// Status.JobName to jobName.
func (s *CloudSchedulerSourceStatus) MarkJobReady(jobName string) {
	schedulerCondSet.Manage(s).MarkTrue(JobReady)
	s.JobName = jobName
}
