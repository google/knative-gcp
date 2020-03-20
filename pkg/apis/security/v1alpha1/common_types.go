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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

// StringMatch defines the specification to match a string.
type StringMatch struct {
	// Exact is to match the exact string.
	Exact string `json:"exact,omitempty"`

	// Prefix is to match the prefix of the string.
	Prefix string `json:"prefix,omitempty"`

	// Suffix is to match the suffix of the string.
	Suffix string `json:"suffix,omitempty"`

	// Presence is to match anything but empty.
	Presence bool `json:"presence,omitempty"`
}

// ToExpression returns the string expression of the string match.
func (m *StringMatch) ToExpression() string {
	if m.Exact != "" {
		return m.Exact
	}
	if m.Prefix != "" {
		return m.Prefix + "*"
	}
	if m.Suffix != "" {
		return "*" + m.Suffix
	}
	if m.Presence {
		return "*"
	}
	return ""
}

// KeyValuesMatch defines a key and a list of string matches for the key.
type KeyValuesMatch struct {
	// Key is a string which could be used to retrieve a value from somewhere.
	Key string `json:"key"`

	// Values is a list of string matches where the value of the key should match.
	Values []StringMatch `json:"values"`
}

// JWTSpec defines the specification to validate JWT.
type JWTSpec struct {
	// JwksURI is the URI of the JWKs for validating JWTs.
	// Can only be specified if Jwks is not set.
	JwksURI string `json:"jwksUri,omitempty"`

	// Jwks is the literal JWKs for validating JWTs.
	// Can only be specified if JwksURI is not specified.
	Jwks string `json:"jwks,omitempty"`

	// Issuer is the issuer of the JWT.
	Issuer string `json:"issuer"`

	// FromHeader is the list of header locations from which JWT is expected.
	FromHeaders []JWTHeader `json:"fromHeaders"`
}

// JWTHeader specifies a header location to extract JWT token.
type JWTHeader struct {
	// Name is the HTTP header name.
	Name string `json:"name"`

	// Prefix is the prefix that should be stripped before decoding the token.
	// E.g. a common one is "Bearer".
	Prefix string `json:"prefix,omitempty"`
}

// JWTRule specifies a rule to check JWT attributes.
type JWTRule struct {
	// Principals is a list of source identities ("iss/sub") to match.
	// If omitted, it implies any principal is allowed.
	Principals []string `json:"principals,omitempty"`

	// Claims is a list of claims that should match certain patterns.
	Claims []KeyValuesMatch `json:"claims,omitempty"`
}

// RequestOperation is the operation the request is taking.
type RequestOperation struct {
	// Hosts is a list of host names to match.
	Hosts []StringMatch `json:"hosts,omitempty"`

	// Paths is a list of paths to match.
	Paths []StringMatch `json:"paths,omitempty"`

	// Methods is a list of methods to match.
	Methods []string `json:"methods,omitempty"`
}

// PolicyBindingSpec is the specification for a policy binding.
type PolicyBindingSpec struct {
	// The binding subject.
	duckv1alpha1.BindingSpec `json:",inline"`

	// Policy is the policy to bind to the subject.
	Policy duckv1.KReference `json:"policy"`
}

// PolicyBindingStatus is the status for a policy binding.
type PolicyBindingStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
}
