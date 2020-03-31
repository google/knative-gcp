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
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPPolicy is the specification for HTTP traffic policy.
type HTTPPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HTTPPolicySpec `json:"spec"`
}

var (
	_ apis.Validatable   = (*HTTPPolicy)(nil)
	_ apis.Defaultable   = (*HTTPPolicy)(nil)
	_ apis.HasSpec       = (*HTTPPolicy)(nil)
	_ runtime.Object     = (*HTTPPolicy)(nil)
	_ kmeta.OwnerRefable = (*HTTPPolicy)(nil)
)

// HTTPPolicySpec is the HTTPPolicy specification.
type HTTPPolicySpec struct {
	// JWT specifies the parameters to validate JTWs.
	// If omitted, authentication will be skipped.
	JWT *JWTSpec `json:"jwt,omitempty"`

	// Rules is the list of rules to check for the policy.
	// The rules should be evaluated in order. If the request under check
	// passes one rule, it passes the policy check.
	// If Rules is not specified, it implies the policy is to "allow all".
	// If an empty rule is specified in Rules, it implies the policy is to "reject all".
	Rules []HTTPPolicyRuleSpec `json:"rules,omitempty"`
}

// HTTPPolicyRuleSpec is the specification for a HTTP policy rule.
// To pass a specified rule, a request must match all attributes provided in the rule.
type HTTPPolicyRuleSpec struct {
	// JWTRule inlines the rule for checking the JWT.
	JWTRule `json:",inline"`

	// Operations is a list of operation attributes to match.
	Operations []RequestOperation `json:"operations,omitempty"`

	// Headers is a list of headers to match.
	Headers []KeyValuesMatch `json:"headers,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPPolicyList is a collection of HTTPPolicy.
type HTTPPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPPolicy `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for HTTPPolicy.
func (p *HTTPPolicy) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("HTTPPolicy")
}

// GetUntypedSpec returns the spec of the HTTPPolicy.
func (p *HTTPPolicy) GetUntypedSpec() interface{} {
	return p.Spec
}
