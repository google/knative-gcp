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
	"github.com/google/knative-gcp/pkg/apis/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
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
		wantErr: apis.ErrMultipleOneOf("exact", "prefix"),
	}, {
		name:    "multiple presence 2",
		m:       StringMatch{Suffix: "abc", Presence: true},
		wantErr: apis.ErrMultipleOneOf("suffix", "presence"),
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
		wantErr: apis.ErrMultipleOneOf("exact", "prefix").ViaFieldIndex("values", 0),
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

func TestValidateJWTSpec(t *testing.T) {
	cases := []struct {
		name    string
		j       JWTSpec
		wantErr *apis.FieldError
	}{{
		name: "valid",
		j: JWTSpec{
			Issuer:      "example.com",
			JwksURI:     "https://example.com",
			FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
		},
	}, {
		name: "valid 2",
		j: JWTSpec{
			Issuer:      "example.com",
			Jwks:        "jwk",
			FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
		},
	}, {
		name: "both jwks and jwksUri are specified",
		j: JWTSpec{
			Issuer:      "example.com",
			Jwks:        "jwk",
			JwksURI:     "https://example.com",
			FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
		},
		wantErr: apis.ErrMultipleOneOf("jwks", "jwksUri"),
	}, {
		name: "neither jwks nor jwksUri is specified",
		j: JWTSpec{
			Issuer:      "example.com",
			FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
		},
		wantErr: apis.ErrMissingOneOf("jwks", "jwksUri"),
	}, {
		name: "missing jwt header",
		j: JWTSpec{
			Issuer: "example.com",
			Jwks:   "jwk",
		},
		wantErr: apis.ErrMissingField("fromHeaders"),
	}, {
		name: "missing issuer",
		j: JWTSpec{
			Jwks:        "jwk",
			FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
		},
		wantErr: apis.ErrMissingField("issuer"),
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

func TestValidateJWTRule(t *testing.T) {
	cases := []struct {
		name    string
		j       JWTRule
		wantErr *apis.FieldError
	}{{
		name: "valid",
		j: JWTRule{
			Principals: []string{"users"},
			Claims:     []KeyValuesMatch{{Key: "iss", Values: []StringMatch{{Prefix: "example.com"}}}},
		},
	}, {
		name: "invalid missing claim key",
		j: JWTRule{
			Principals: []string{"users"},
			Claims:     []KeyValuesMatch{{Values: []StringMatch{{Prefix: "example.com"}}}},
		},
		wantErr: apis.ErrMissingField("key").ViaFieldIndex("claims", 0),
	}, {
		name: "invalid claim value",
		j: JWTRule{
			Principals: []string{"users"},
			Claims:     []KeyValuesMatch{{Key: "iss", Values: []StringMatch{{Exact: "example", Prefix: "example.com"}}}},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix").ViaFieldIndex("values", 0).ViaFieldIndex("claims", 0),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.j.Validate(context.Background())
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("JWTRule.Validate (-want, +got) = %v", diff)
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
		wantErr: apis.ErrMultipleOneOf("prefix", "suffix").ViaFieldIndex("parent", 1),
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
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "foo",
				},
			},
			Policy: duckv1.KReference{
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
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "bar",
				},
			},
			Policy: duckv1.KReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrInvalidValue("bar", "namespace").ViaField("subject"),
	}, {
		name: "subject name and selector both specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "foo",
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrMultipleOneOf("name", "selector").ViaField("subject"),
	}, {
		name: "subject reference partially specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Name:      "subject",
					Namespace: "foo",
				},
			},
			Policy: duckv1.KReference{
				Name:      "policy",
				Namespace: "foo",
			},
		},
		parentNamespace: "foo",
		wantErr:         subjectRefErr.ViaField("subject"),
	}, {
		name: "subject name and selector not specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Namespace: "foo",
				},
			},
			Policy: duckv1.KReference{
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
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "foo",
				},
			},
			Policy: duckv1.KReference{
				Name:      "policy",
				Namespace: "bar",
			},
		},
		parentNamespace: "foo",
		wantErr:         apis.ErrInvalidValue("bar", "namespace").ViaField("policy"),
	}, {
		name: "policy API specified",
		spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "foo",
				},
			},
			Policy: duckv1.KReference{
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
					APIVersion: "example.com/v1",
					Kind:       "Foo",
					Name:       "subject",
					Namespace:  "foo",
				},
			},
			Policy: duckv1.KReference{
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

func TestPolicyBindingSpecCheckImmutableFields(t *testing.T) {
	cases := []struct {
		name    string
		orignal *PolicyBindingSpec
		updated *PolicyBindingSpec
		wantErr *apis.FieldError
	}{{
		name: "subject changed",
		orignal: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "policy",
			},
		},
		updated: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "foo"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "policy",
			},
		},
		wantErr: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: "{*v1alpha1.PolicyBindingSpec}.BindingSpec.Subject.Selector.MatchLabels[\"app\"]:\n\t-: \"test\"\n\t+: \"foo\"\n",
		},
	}, {
		name: "policy changed",
		orignal: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "policy",
			},
		},
		updated: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "new-policy",
			},
		},
		wantErr: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: "{*v1alpha1.PolicyBindingSpec}.Policy.Name:\n\t-: \"policy\"\n\t+: \"new-policy\"\n",
		},
	}, {
		name: "not changed",
		orignal: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "policy",
			},
		},
		updated: &PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
				},
			},
			Policy: duckv1.KReference{
				Name: "policy",
			},
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.updated.CheckImmutableFields(context.Background(), tc.orignal)
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("PolicyBindingSpec.CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}

func TestCheckPolicyBindingImmutableObjectMeta(t *testing.T) {
	m1 := &metav1.ObjectMeta{
		Annotations: map[string]string{
			policy.PolicyBindingClassAnnotationKey: "foo",
		},
	}
	m2 := &metav1.ObjectMeta{
		Annotations: map[string]string{
			policy.PolicyBindingClassAnnotationKey: "bar",
		},
	}

	wantErr := &apis.FieldError{
		Message: "Immutable fields changed (-old +new)",
		Paths:   []string{"annotations", policy.PolicyBindingClassAnnotationKey},
		Details: "-: \"foo\"\n+: \"bar\"",
	}
	gotErr := CheckImmutableBindingObjectMeta(context.Background(), m2, m1)
	if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
		t.Errorf("CheckImmutableBindingObjectMeta (-want, +got) = %v", diff)
	}
}
