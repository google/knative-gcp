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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// Identity is an Implementable "duck type".
var _ duck.Implementable = (*Identity)(nil)

// +genduck
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Identity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdentitySpec   `json:"spec"`
	Status IdentityStatus `json:"status"`
}

type IdentitySpec struct {
	// ServiceAccount is the GCP service account which has required permissions to poll from a Cloud Pub/Sub subscription.
	// If not specified, defaults to use secret.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// IdentityStatus inherits duck/v1 Status and adds a ServiceAccountName.
type IdentityStatus struct {
	// Inherits duck/v1 Status,, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
	// ServiceAccountName is the k8s service account associated with Google service account.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	IdentityConfigured apis.ConditionType = "WorkloadIdentityConfigured"
)

// IsReady returns true if the resource is ready overall.
func (ss *IdentityStatus) IsReady() bool {
	for _, c := range ss.Conditions {
		switch c.Type {
		// Look for the "happy" condition, which is the only condition that
		// we can reliably understand to be the overall state of the resource.
		case apis.ConditionReady, apis.ConditionSucceeded:
			return c.IsTrue()
		}
	}
	return false
}

var (
	// Verify Identity resources meet duck contracts.
	_ duck.Populatable = (*Identity)(nil)
	_ apis.Listable    = (*Identity)(nil)
)

// GetFullType implements duck.Implementable
func (*Identity) GetFullType() duck.Populatable {
	return &Identity{}
}

// Populate implements duck.Populatable
func (s *Identity) Populate() {
	s.Spec.ServiceAccount = ""
}

// GetListType implements apis.Listable
func (*Identity) GetListType() runtime.Object {
	return &IdentityList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IdentityList is a list of Identity resources
type IdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Identity `json:"items"`
}
