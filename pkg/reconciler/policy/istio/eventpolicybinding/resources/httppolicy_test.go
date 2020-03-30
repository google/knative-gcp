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

package resources

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

func TestMakeHTTPPolicy(t *testing.T) {
	eb := &v1alpha1.EventPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "testnamespace",
		},
		Spec: v1alpha1.PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			Policy: duckv1.KReference{
				Name:      "test-event-policy",
				Namespace: "testnamespace",
			},
		},
	}
	ep := &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event-policy",
			Namespace: "testnamespace",
		},
		Spec: v1alpha1.EventPolicySpec{
			JWT: &v1alpha1.JWTSpec{
				JwksURI: "https://example.com/jwks.json",
				Issuer:  "example.com",
				FromHeaders: []v1alpha1.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			},
			Rules: []v1alpha1.EventPolicyRuleSpec{
				{
					JWTRule: v1alpha1.JWTRule{
						Principals: []string{"user-a@example.com"},
					},
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Methods: []string{"GET", "POST"},
							Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
						},
					},
					ID:          []v1alpha1.StringMatch{{Exact: "001"}},
					Source:      []v1alpha1.StringMatch{{Suffix: "example.com"}},
					Subject:     []v1alpha1.StringMatch{{Presence: true}},
					Type:        []v1alpha1.StringMatch{{Prefix: "hello"}},
					ContentType: []v1alpha1.StringMatch{{Exact: "application/json"}},
					Extensions: []v1alpha1.KeyValuesMatch{
						{Key: "custom1", Values: []v1alpha1.StringMatch{{Exact: "foo"}}},
						{Key: "custom2", Values: []v1alpha1.StringMatch{{Exact: "bar"}}},
					},
				},
			},
		},
	}

	wantPolicy := &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName("test-event-policy", "-http"),
			Namespace:       "testnamespace",
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(eb)},
		},
		Spec: v1alpha1.HTTPPolicySpec{
			JWT: &v1alpha1.JWTSpec{
				JwksURI: "https://example.com/jwks.json",
				Issuer:  "example.com",
				FromHeaders: []v1alpha1.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			},
			Rules: []v1alpha1.HTTPPolicyRuleSpec{
				{
					JWTRule: v1alpha1.JWTRule{
						Principals: []string{"user-a@example.com"},
					},
					Operations: []v1alpha1.RequestOperation{
						{
							Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
							Methods: []string{"GET", "POST"},
							Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
						},
					},
					Headers: []v1alpha1.KeyValuesMatch{
						{Key: "ce-id", Values: []v1alpha1.StringMatch{{Exact: "001"}}},
						{Key: "ce-source", Values: []v1alpha1.StringMatch{{Suffix: "example.com"}}},
						{Key: "ce-type", Values: []v1alpha1.StringMatch{{Prefix: "hello"}}},
						{Key: "ce-subject", Values: []v1alpha1.StringMatch{{Presence: true}}},
						{Key: "Content-Type", Values: []v1alpha1.StringMatch{{Exact: "application/json"}}},
						{Key: "ce-custom1", Values: []v1alpha1.StringMatch{{Exact: "foo"}}},
						{Key: "ce-custom2", Values: []v1alpha1.StringMatch{{Exact: "bar"}}},
					},
				},
			},
		},
	}

	gotPolicy := MakeHTTPPolicy(eb, ep)
	if diff := cmp.Diff(wantPolicy, gotPolicy); diff != "" {
		t.Errorf("MakeHTTPPolicy (-want,+got): %v", diff)
	}
}
