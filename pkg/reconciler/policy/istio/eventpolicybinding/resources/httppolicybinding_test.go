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
	"github.com/google/knative-gcp/pkg/apis/policy"
	"github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
)

func TestMakeHTTPPolicyBinding(t *testing.T) {
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
	hp := &v1alpha1.HTTPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-http-policy",
			Namespace: "testnamespace",
		},
	}

	wantBinding := &v1alpha1.HTTPPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kmeta.ChildName("test-binding", "-httpbinding"),
			Namespace:       "testnamespace",
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(eb)},
			Annotations: map[string]string{
				policy.PolicyBindingClassAnnotationKey: policy.IstioPolicyBindingClassValue,
			},
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
				Name:      "test-http-policy",
				Namespace: "testnamespace",
			},
		},
	}

	gotBinding := MakeHTTPPolicyBinding(eb, hp)
	if diff := cmp.Diff(wantBinding, gotBinding); diff != "" {
		t.Errorf("MakeHTTPPolicyBinding (-want,+got): %v", diff)
	}
}
