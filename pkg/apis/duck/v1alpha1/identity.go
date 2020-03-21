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

const (
	WorkloadIdentityFailed        = "WorkloadIdentityReconcileFailed"
	WorkloadIdentitySucceed       = "WorkloadIdentityReconciled"
	DeleteWorkloadIdentityFailed  = "WorkloadIdentityDeleteFailed"
	DeleteWorkloadIdentitySucceed = "WorkloadIdentityDeleted"
)

// WorkloadIdentityStatus represents the status of WorkloadIdentity.
type WorkloadIdentityStatus struct {
	// Enabled represents whether workload identity is enabled or not.
	Enabled string `json:"enabled,omitempty"`

	// Ready represents whether workload identity reconciles successfully or not. If it is not enabled, Ready will be unknown.
	Ready string `json:"ready,omitempty"`

	// Short reason for the status.
	Reason string `json:"reason,omitempty"`

	// A human readable message indicating details about the failure.
	Message string `json:"message,omitempty"`

	// The kubernetes service account which is bound to the provided GCP service account.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// InitWorkloadIdentityStatus assumes workload identity is not enabled.
func (wis *WorkloadIdentityStatus) InitWorkloadIdentityStatus() {
	wis.SetWorkloadIdentityStatus("False", "Unknown", "", "", "")
}

// InitReconcileWorkloadIdentityStatus assumes reconciling workload identity is failed.
func (wis *WorkloadIdentityStatus) InitReconcileWorkloadIdentityStatus() {
	wis.SetWorkloadIdentityStatus("True", "False", WorkloadIdentityFailed, "", "")
}

// InitDeleteWorkloadIdentityStatus assumes deleting workload identity is failed.
func (wis *WorkloadIdentityStatus) InitDeleteWorkloadIdentityStatus() {
	wis.SetWorkloadIdentityStatus("True", "False", DeleteWorkloadIdentityFailed, "", "")
}

func (wis *WorkloadIdentityStatus) MarkWorkloadIdentityReconciled() {
	serviceAccountName := wis.ServiceAccountName
	wis.SetWorkloadIdentityStatus("True", "True", WorkloadIdentitySucceed, "", serviceAccountName)
}

func (wis *WorkloadIdentityStatus) MarkWorkloadIdentityDeleted() {
	serviceAccountName := wis.ServiceAccountName
	wis.SetWorkloadIdentityStatus("True", "True", DeleteWorkloadIdentitySucceed, "", serviceAccountName)
}

func (wis *WorkloadIdentityStatus) SetWorkloadIdentityStatus(enabled, ready, reason, message, serviceAccountName string) {
	wis.Enabled = enabled
	wis.Ready = ready
	wis.Reason = reason
	wis.Message = message
	wis.ServiceAccountName = serviceAccountName
}
