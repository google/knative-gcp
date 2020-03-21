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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestHTTPPolicyValidation(t *testing.T) {
	cases := []struct {
		name    string
		p       HTTPPolicy
		wantErr *apis.FieldError
	}{{
		name: "valid",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping"}},
							},
						},
					},
				},
			},
		},
	}, {
		name: "invalid jwt",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{Issuer: "example.com", FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}}},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping"}},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrMissingOneOf("jwks", "jwksUri").ViaField("jwt").ViaField("spec"),
	}, {
		name: "invalid claim",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping"}},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrMissingField("key").ViaFieldIndex("claims", 0).ViaFieldIndex("rules", 0).ViaField("spec"),
	}, {
		name: "invalid header",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping"}},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrMissingField("key").ViaFieldIndex("headers", 0).ViaFieldIndex("rules", 0).ViaField("spec"),
	}, {
		name: "invalid host",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local", Exact: "abc"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping"}},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "suffix").ViaFieldIndex("hosts", 0).ViaFieldIndex("operations", 0).ViaFieldIndex("rules", 0).ViaField("spec"),
	}, {
		name: "invalid path",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"POST"},
								Paths:   []StringMatch{{Prefix: "/ping", Exact: "abc"}},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrMultipleOneOf("exact", "prefix").ViaFieldIndex("paths", 0).ViaFieldIndex("operations", 0).ViaFieldIndex("rules", 0).ViaField("spec"),
	}, {
		name: "invalid method",
		p: HTTPPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "my-policy"},
			Spec: HTTPPolicySpec{
				JWT: &JWTSpec{
					Issuer:      "example.com",
					Jwks:        "jwks",
					FromHeaders: []JWTHeader{{Name: "Authorization", Prefix: "Bearer"}},
				},
				Rules: []HTTPPolicyRuleSpec{
					{
						JWTRule: JWTRule{
							Principals: []string{"user"},
							Claims:     []KeyValuesMatch{{Key: "aud", Values: []StringMatch{{Exact: "me"}}}},
						},
						Headers: []KeyValuesMatch{{Key: "foo", Values: []StringMatch{{Exact: "bar"}}}},
						Operations: []RequestOperation{
							{
								Hosts:   []StringMatch{{Suffix: "svc.cluster.local"}},
								Methods: []string{"XXX"},
							},
						},
					},
				},
			},
		},
		wantErr: apis.ErrInvalidArrayValue("XXX", "methods", 0).ViaFieldIndex("operations", 0).ViaFieldIndex("rules", 0).ViaField("spec"),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.p.Validate(context.Background())
			if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("HTTPPolicy.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
