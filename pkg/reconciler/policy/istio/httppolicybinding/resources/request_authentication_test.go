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

const (
	testBindingName = "testpolicybinding"
	testNamespace   = "testnamespace"
	testJwksURI     = "https://example.com/jwks.json"
)

func TestMakeRequestAuthentication(t *testing.T) {
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
	jwt := v1alpha1.JWTSpec{
		JwksURI: testJwksURI,
		Issuer:  "example.com",
		FromHeaders: []v1alpha1.JWTHeader{
			{Name: "Authorization", Prefix: "Bearer"},
			{Name: "X-Custom-Token"},
		},
	}
	wantReqAuthn := istioclient.RequestAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(b.Name, "-req-authn"),
			Namespace:       testNamespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
		},
		Spec: istiopolicy.RequestAuthentication{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			JwtRules: []*istiopolicy.JWTRule{{
				Issuer:               "example.com",
				JwksUri:              testJwksURI,
				ForwardOriginalToken: true,
				FromHeaders: []*istiopolicy.JWTHeader{
					{Name: "Authorization", Prefix: "Bearer"},
					{Name: "X-Custom-Token"},
				},
			}},
		},
	}

	gotReqAuthn := MakeRequestAuthentication(b, selector, jwt)
	if diff := cmp.Diff(wantReqAuthn, gotReqAuthn); diff != "" {
		t.Errorf("MakeRequestAuthentication (-want,+got): %v", diff)
	}
}
