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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	"github.com/google/knative-gcp/pkg/apis/policy"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

// MakeHTTPPolicyBinding generates a HTTPPolicyBinding based on the given EventPolicyBinding.
func MakeHTTPPolicyBinding(b *v1alpha1.EventPolicyBinding, hp *v1alpha1.HTTPPolicy) *v1alpha1.HTTPPolicyBinding {
	hb := &v1alpha1.HTTPPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName(b.Name, "-httpbinding"),
			Namespace:       b.Namespace,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(b)},
			Annotations: map[string]string{
				policy.PolicyBindingClassAnnotationKey: policy.IstioPolicyBindingClassValue,
			},
		},
		Spec: v1alpha1.PolicyBindingSpec{
			BindingSpec: b.Spec.BindingSpec,
			Policy: duckv1.KReference{
				Name:      hp.Name,
				Namespace: hp.Namespace,
			},
		},
	}
	return hb
}
