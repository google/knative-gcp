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
	istiopolicy "istio.io/api/security/v1beta1"
	istiotype "istio.io/api/type/v1beta1"
	istioclient "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

// MakeRequestAuthentication makes an Istio RequestAuthentication.
// Reference: https://istio.io/docs/reference/config/policy/request_authentication/
func MakeRequestAuthentication(
	b *v1alpha1.HTTPPolicyBinding,
	subjectSelector *metav1.LabelSelector,
	jwt v1alpha1.JWTSpec) istioclient.RequestAuthentication {

	var rhs []*istiopolicy.JWTHeader
	for _, rh := range jwt.FromHeaders {
		rhs = append(rhs, &istiopolicy.JWTHeader{Name: rh.Name, Prefix: rh.Prefix})
	}

	return istioclient.RequestAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(b.Name, "-req-authn"),
			Namespace:       b.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
		},
		Spec: istiopolicy.RequestAuthentication{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: subjectSelector.MatchLabels,
			},
			JwtRules: []*istiopolicy.JWTRule{{
				Issuer:               jwt.Issuer,
				Jwks:                 jwt.Jwks,
				JwksUri:              jwt.JwksURI,
				ForwardOriginalToken: true,
				FromHeaders:          rhs,
			}},
		},
	}
}
