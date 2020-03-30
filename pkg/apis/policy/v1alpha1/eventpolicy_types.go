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

// EventPolicy is a specification for cloudevent traffic policy.
type EventPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec EventPolicySpec `json:"spec"`
}

var (
	_ apis.Validatable   = (*EventPolicy)(nil)
	_ apis.Defaultable   = (*EventPolicy)(nil)
	_ apis.HasSpec       = (*EventPolicy)(nil)
	_ runtime.Object     = (*EventPolicy)(nil)
	_ kmeta.OwnerRefable = (*EventPolicy)(nil)
)

// EventPolicySpec is the specification for EventPolicy.
type EventPolicySpec struct {
	// JWT specifies the parameters to validate JTWs.
	// If omitted, authentication will be skipped.
	JWT *JWTSpec `json:"jwt,omitempty"`

	// Rules is the list of rules to check for the policy.
	// The rules should be evaluated in order. If the request under check
	// passes one rule, it passes the policy check.
	// If Rules is not specified, it implies the policy is to "allow all".
	// If an empty rule is specified in Rules, it implies the policy is to "reject all".
	Rules []EventPolicyRuleSpec `json:"rules,omitempty"`
}

// EventPolicyRuleSpec defines the rule specification for EventPolicy.
type EventPolicyRuleSpec struct {
	// JWTRule inlines the rule for checking the JWT.
	JWTRule `json:",inline"`

	// Operations is a list of operation attributes to match.
	Operations []RequestOperation `json:"operations,omitempty"`

	// The following attributes map to standard CloudEvents attributes.

	ID          []StringMatch    `json:"id,omitempty"`
	Source      []StringMatch    `json:"source,omitempty"`
	Type        []StringMatch    `json:"type,omitempty"`
	DataSchema  []StringMatch    `json:"dataschema,omitempty"`
	Subject     []StringMatch    `json:"subject,omitempty"`
	ContentType []StringMatch    `json:"contenttype,omitempty"`
	Extensions  []KeyValuesMatch `json:"extensions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventPolicyList is a collection of EventPolicy.
type EventPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventPolicy `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventPolicy
func (p *EventPolicy) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("EventPolicy")
}

// GetUntypedSpec returns the spec of the EventPolicy.
func (p *EventPolicy) GetUntypedSpec() interface{} {
	return p.Spec
}
