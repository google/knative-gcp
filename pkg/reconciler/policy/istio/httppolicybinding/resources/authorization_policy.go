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
	"fmt"

	istiopolicy "istio.io/api/security/v1beta1"
	istiotype "istio.io/api/type/v1beta1"
	istioclient "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

// See: https://istio.io/docs/reference/config/policy/conditions/
const (
	istioClaimKeyPattern  = "request.auth.claims[%s]"
	istioHeaderKeyPattern = "request.headers[%s]"
)

// MakeAuthorizationPolicy makes an Istio AuthorizationPolicy.
// Reference: https://istio.io/docs/reference/config/policy/authorization-policy/
func MakeAuthorizationPolicy(
	b *v1alpha1.HTTPPolicyBinding,
	subjectSelector *metav1.LabelSelector,
	rules []v1alpha1.HTTPPolicyRuleSpec) istioclient.AuthorizationPolicy {

	var rs []*istiopolicy.Rule
	for _, r := range rules {
		var rfs []*istiopolicy.Rule_From
		if len(r.Principals) > 0 {
			rfs = []*istiopolicy.Rule_From{{
				Source: &istiopolicy.Source{
					RequestPrincipals: r.Principals,
				},
			}}
		}

		var rts []*istiopolicy.Rule_To
		for _, op := range r.Operations {
			rt := &istiopolicy.Rule_To{
				Operation: &istiopolicy.Operation{},
			}
			for _, h := range op.Hosts {
				rt.Operation.Hosts = append(rt.Operation.Hosts, h.ToExpression())
			}
			for _, p := range op.Paths {
				rt.Operation.Paths = append(rt.Operation.Paths, p.ToExpression())
			}
			for _, m := range op.Methods {
				rt.Operation.Methods = append(rt.Operation.Methods, m)
			}
			rts = append(rts, rt)
		}

		var rcs []*istiopolicy.Condition
		for _, cl := range r.Claims {
			rc := &istiopolicy.Condition{
				Key: istioClaimKey(cl.Key),
			}
			for _, v := range cl.Values {
				rc.Values = append(rc.Values, v.ToExpression())
			}
			rcs = append(rcs, rc)
		}
		for _, h := range r.Headers {
			rc := &istiopolicy.Condition{
				Key: istioHeaderKey(h.Key),
			}
			for _, v := range h.Values {
				rc.Values = append(rc.Values, v.ToExpression())
			}
			rcs = append(rcs, rc)
		}

		rs = append(rs, &istiopolicy.Rule{
			From: rfs,
			To:   rts,
			When: rcs,
		})
	}

	return istioclient.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(b.Name, "-authz"),
			Namespace:       b.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
		},
		Spec: istiopolicy.AuthorizationPolicy{
			Selector: &istiotype.WorkloadSelector{
				MatchLabels: subjectSelector.MatchLabels,
			},
			Action: istiopolicy.AuthorizationPolicy_ALLOW,
			Rules:  rs,
		},
	}
}

func istioClaimKey(cl string) string {
	return fmt.Sprintf(istioClaimKeyPattern, cl)
}

func istioHeaderKey(h string) string {
	return fmt.Sprintf(istioHeaderKeyPattern, h)
}
