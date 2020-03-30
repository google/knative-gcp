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

	istiopolicy "istio.io/api/security/v1beta1"
	istiotype "istio.io/api/type/v1beta1"
	istioclient "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

func TestMakeAuthorizationPolicy(t *testing.T) {
	b := &v1alpha1.HTTPPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBindingName,
			Namespace: testNamespace,
		},
	}
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "test",
		},
	}
	rules := []v1alpha1.HTTPPolicyRuleSpec{
		{
			JWTRule: v1alpha1.JWTRule{
				Principals: []string{"user-a@example.com"},
				Claims: []v1alpha1.KeyValuesMatch{
					{Key: "iss", Values: []v1alpha1.StringMatch{{Exact: "https://example.com"}}},
					{Key: "aud", Values: []v1alpha1.StringMatch{{Suffix: ".svc.cluster.local"}}},
				},
			},
			Headers: []v1alpha1.KeyValuesMatch{
				{Key: "K-test", Values: []v1alpha1.StringMatch{{Exact: "val1"}, {Prefix: "foo-"}}},
				{Key: "K-must-present", Values: []v1alpha1.StringMatch{{Presence: true}}},
			},
			Operations: []v1alpha1.RequestOperation{
				{
					Hosts:   []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
					Methods: []string{"GET", "POST"},
					Paths:   []v1alpha1.StringMatch{{Prefix: "/operation/"}, {Prefix: "/admin/"}},
				},
			},
		},
		{
			Operations: []v1alpha1.RequestOperation{
				{
					Hosts: []v1alpha1.StringMatch{{Suffix: ".mysvc.svc.cluster.local"}},
					Paths: []v1alpha1.StringMatch{{Prefix: "/public/"}},
				},
			},
		},
	}
	wantAuthzPolicy := istioclient.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(b.Name, "-authz"),
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
		},
		Spec: istiopolicy.AuthorizationPolicy{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Action: istiopolicy.AuthorizationPolicy_ALLOW,
			Rules: []*istiopolicy.Rule{
				{
					From: []*istiopolicy.Rule_From{
						{Source: &istiopolicy.Source{RequestPrincipals: []string{"user-a@example.com"}}},
					},
					To: []*istiopolicy.Rule_To{
						{Operation: &istiopolicy.Operation{
							Hosts:   []string{"*.mysvc.svc.cluster.local"},
							Methods: []string{"GET", "POST"},
							Paths:   []string{"/operation/*", "/admin/*"},
						}},
					},
					When: []*istiopolicy.Condition{
						{
							Key:    "request.auth.claims[iss]",
							Values: []string{"https://example.com"},
						},
						{
							Key:    "request.auth.claims[aud]",
							Values: []string{"*.svc.cluster.local"},
						},
						{
							Key:    "request.headers[K-test]",
							Values: []string{"val1", "foo-*"},
						},
						{
							Key:    "request.headers[K-must-present]",
							Values: []string{"*"},
						},
					},
				},
				{
					To: []*istiopolicy.Rule_To{{
						Operation: &istiopolicy.Operation{
							Hosts: []string{"*.mysvc.svc.cluster.local"},
							Paths: []string{"/public/*"},
						},
					}},
				},
			},
		},
	}

	gotAuthzPolicy := MakeAuthorizationPolicy(b, selector, rules)
	if diff := cmp.Diff(wantAuthzPolicy, gotAuthzPolicy); diff != "" {
		t.Errorf("MakeAuthorizationPolicy (-want,+got): %v", diff)
	}
}
