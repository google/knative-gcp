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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"
)

func TestValidateStringMatch(t *testing.T) {
	cases := []struct {
		name    string
		m       StringMatch
		wantErr *apis.FieldError
	}{{
		name: "exact",
		m:    StringMatch{Exact: "abc"},
	}, {
		name: "prefix",
		m:    StringMatch{Prefix: "abc"},
	}, {
		name: "suffix",
		m:    StringMatch{Suffix: "abc"},
	}, {
		name: "presence",
		m:    StringMatch{Presence: true},
	}, {
		name:    "multiple presence",
		m:       StringMatch{Exact: "abc", Prefix: "xxx"},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence"),
	}, {
		name:    "multiple presence 2",
		m:       StringMatch{Suffix: "abc", Presence: true},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence"),
	}, {
		name:    "not set",
		m:       StringMatch{},
		wantErr: apis.ErrMissingOneOf("exact", "prefix", "suffix", "presence"),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.m.Validate(context.Background())
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("StringMatch.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidationKeyValuesMatch(t *testing.T) {
	cases := []struct {
		name    string
		kvm     KeyValuesMatch
		wantErr *apis.FieldError
	}{{
		name: "valid",
		kvm:  KeyValuesMatch{Key: "foo", Values: []StringMatch{{Exact: "bar"}}},
	}, {
		name:    "key missing",
		kvm:     KeyValuesMatch{Values: []StringMatch{{Exact: "bar"}}},
		wantErr: apis.ErrMissingField("key"),
	}, {
		name:    "value missing",
		kvm:     KeyValuesMatch{Key: "foo"},
		wantErr: apis.ErrMissingField("values"),
	}, {
		name:    "invalid values",
		kvm:     KeyValuesMatch{Key: "foo", Values: []StringMatch{{Exact: "bar", Prefix: "abc"}}},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence").ViaFieldIndex("values", 0),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.kvm.Validate(context.Background())
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("KeyValuesMatch.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	cases := []struct {
		name    string
		j       JWTSpec
		wantErr *apis.FieldError
	}{{
		name: "valid",
		j: JWTSpec{
			JwksURI:      "https://example.com",
			JwtHeader:    "Authorization",
			ExcludePaths: []StringMatch{{Prefix: "/exclude/"}},
		},
	}, {
		name: "valid 2",
		j: JWTSpec{
			Jwks:         "jwk",
			JwtHeader:    "Authorization",
			IncludePaths: []StringMatch{{Prefix: "/include/"}},
		},
	}, {
		name: "both jwks and jwksUri are specified",
		j: JWTSpec{
			Jwks:      "jwk",
			JwksURI:   "https://example.com",
			JwtHeader: "Authorization",
		},
		wantErr: apis.ErrMultipleOneOf("jwks", "jwksUri"),
	}, {
		name: "neither jwks nor jwksUri is specified",
		j: JWTSpec{
			JwtHeader:    "Authorization",
			IncludePaths: []StringMatch{{Prefix: "/include/"}},
		},
		wantErr: apis.ErrMissingOneOf("jwks", "jwksUri"),
	}, {
		name: "missing jwt header",
		j: JWTSpec{
			Jwks: "jwk",
		},
		wantErr: apis.ErrMissingField("jwtHeader"),
	}, {
		name: "invalid include paths",
		j: JWTSpec{
			Jwks:         "jwk",
			JwtHeader:    "Authorization",
			IncludePaths: []StringMatch{{Prefix: "/include/", Exact: "abc"}},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence").ViaFieldIndex("includePaths", 0),
	}, {
		name: "invalid include paths",
		j: JWTSpec{
			Jwks:         "jwk",
			JwtHeader:    "Authorization",
			ExcludePaths: []StringMatch{{Prefix: "/exclude/", Exact: "abc"}},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence").ViaFieldIndex("excludePaths", 0),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.j.Validate(context.Background())
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("JWTSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidateStringMatches(t *testing.T) {
	cases := []struct {
		name    string
		sm      []StringMatch
		wantErr *apis.FieldError
	}{{
		name: "nil",
	}, {
		name: "valid",
		sm: []StringMatch{
			{Exact: "abc"},
			{Prefix: "p-"},
			{Suffix: "-s"},
		},
	}, {
		name: "invalid",
		sm: []StringMatch{
			{Exact: "abc"},
			{Suffix: "-s", Prefix: "p-"},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix", "suffix", "presence").ViaFieldIndex("parent", 1),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := ValidateStringMatches(context.Background(), tc.sm, "parent")
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("ValidateStringMatches (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidatePolicyBindingSpec(t *testing.T) {
	cases := []struct {
		name            string
		spec            PolicyBindingSpec
		parentNamespace string
		wantErr         *apis.FieldError
	}{{
		name: "valid",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
			Policy: &corev1.ObjectReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
	}, {
		name: "subject namespace mismatch",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "bar",
				},
			},
			Policy: &corev1.ObjectReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrInvalidValue("bar", "namespace").ViaField("subject"),
	}, {
		name: "subject name and selector not specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Namespace: "foo",
				},
			},
			Policy: &corev1.ObjectReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrMissingOneOf("name", "selector").ViaField("subject"),
	}, {
		name: "policy namespace mismatch",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
			Policy: &corev1.ObjectReference{
				Name:      "policy",
				Namespace: "bar",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrInvalidValue("bar", "namespace").ViaField("policy"),
	}, {
		name: "policy missing",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrMissingField("policy"),
	}, {
		name: "policy API specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
			Policy: &corev1.ObjectReference{
				APIVersion: "other.policy",
				Name:       "policy",
				Namespace:  "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrDisallowedFields("apiVersion", "kind").ViaField("policy"),
	}, {
		name: "policy kind specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
			Policy: &corev1.ObjectReference{
				Kind:      "other.kind",
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrDisallowedFields("apiVersion", "kind").ViaField("policy"),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.spec.Validate(context.Background(), tc.parentNamespace)
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("PolicyBindingSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
