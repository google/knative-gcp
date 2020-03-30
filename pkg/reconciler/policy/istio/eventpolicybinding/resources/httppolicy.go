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
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

// MakeHTTPPolicy generates a HTTPPolicy based on the given EventPolicy.
func MakeHTTPPolicy(b *v1alpha1.EventPolicyBinding, p *v1alpha1.EventPolicy) *v1alpha1.HTTPPolicy {
	hp := &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(p.Name, "-http"),
			Namespace:       p.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
		},
		Spec: v1alpha1.HTTPPolicySpec{
			JWT: p.Spec.JWT,
		},
	}
	for _, r := range p.Spec.Rules {
		hr := v1alpha1.HTTPPolicyRuleSpec{
			JWTRule:    r.JWTRule,
			Operations: r.Operations,
		}

		hr.Headers = appendEventAttributeToHeaders(hr.Headers, "id", r.ID)
		hr.Headers = appendEventAttributeToHeaders(hr.Headers, "source", r.Source)
		hr.Headers = appendEventAttributeToHeaders(hr.Headers, "type", r.Type)
		hr.Headers = appendEventAttributeToHeaders(hr.Headers, "subject", r.Subject)
		hr.Headers = appendEventAttributeToHeaders(hr.Headers, "dataschema", r.DataSchema)

		// Content type should be the native HTTP content type header.
		// https://github.com/cloudevents/spec/blob/master/http-protocol-binding.md#311-http-content-type
		if r.ContentType != nil {
			hr.Headers = append(hr.Headers, v1alpha1.KeyValuesMatch{
				Key:    cehttp.ContentType,
				Values: r.ContentType,
			})
		}

		for _, ext := range r.Extensions {
			hr.Headers = appendEventAttributeToHeaders(hr.Headers, ext.Key, ext.Values)
		}

		hp.Spec.Rules = append(hp.Spec.Rules, hr)
	}
	return hp
}

func appendEventAttributeToHeaders(headers []v1alpha1.KeyValuesMatch, key string, values []v1alpha1.StringMatch) []v1alpha1.KeyValuesMatch {
	if values == nil {
		return headers
	}
	return append(headers, v1alpha1.KeyValuesMatch{
		Key:    "ce-" + key,
		Values: values,
	})
}
