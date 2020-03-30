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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPPolicyBinding is the binding of a HTTP policy to a subject.
type HTTPPolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyBindingSpec   `json:"spec"`
	Status PolicyBindingStatus `json:"status"`
}

var (
	_ apis.Validatable   = (*HTTPPolicyBinding)(nil)
	_ apis.Defaultable   = (*HTTPPolicyBinding)(nil)
	_ apis.HasSpec       = (*HTTPPolicyBinding)(nil)
	_ runtime.Object     = (*HTTPPolicyBinding)(nil)
	_ kmeta.OwnerRefable = (*HTTPPolicyBinding)(nil)
	_ duck.Bindable      = (*HTTPPolicyBinding)(nil)
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPPolicyBindingList is a collection of HTTPPolicyBindings.
type HTTPPolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPPolicyBinding `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for HTTPPolicyBinding.
func (p *HTTPPolicyBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("HTTPPolicyBinding")
}

// GetUntypedSpec returns the spec of the HTTPPolicyBinding.
func (p *HTTPPolicyBinding) GetUntypedSpec() interface{} {
	return p.Spec
}

// GetSubject returns the standard Binding duck's "Subject" field.
// This implements duck.Bindable.
func (p *HTTPPolicyBinding) GetSubject() tracker.Reference {
	return p.Spec.Subject
}

// GetBindingStatus returns the status of the Binding, which must
// implement BindableStatus.
// This implements duck.Bindable.
func (p *HTTPPolicyBinding) GetBindingStatus() duck.BindableStatus {
	return &p.Status
}
