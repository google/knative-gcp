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

package policy

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/policy"
	"github.com/google/knative-gcp/pkg/client/clientset/versioned/scheme"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/tracker"
)

const (
	resourceAPIVersion = "duck.knative.dev/v1"
	resourceKind       = "KResource"
	resourceName       = "testresource"
	parentName         = "testparent"
	testNS             = "testnamespace"
)

func TestResolveSubject(t *testing.T) {
	cases := []struct {
		name         string
		objects      []runtime.Object
		subject      tracker.Reference
		wantSelector *metav1.LabelSelector
		wantErr      bool
	}{{
		name: "subject is label selector",
		subject: tracker.Reference{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
		},
		wantSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
	}, {
		name: "subject not found",
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantErr: true,
	}, {
		name: "subject not authorizable",
		objects: []runtime.Object{
			genObject("", nil),
		},
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantErr: true,
	}, {
		name: "invalid authorizable annotation",
		objects: []runtime.Object{
			genObject("random", nil),
		},
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantErr: true,
	}, {
		name: "self authorizable without labels",
		objects: []runtime.Object{
			genObject("self", nil),
		},
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantErr: true,
	}, {
		name: "self authorizable with labels",
		objects: []runtime.Object{
			genObject("self", map[string]interface{}{"app": "test"}),
		},
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
	}, {
		name: "valid authorizable annotation",
		objects: []runtime.Object{
			genObject(`{"matchLabels":{"app":"test"}}`, nil),
		},
		subject: tracker.Reference{
			APIVersion: resourceAPIVersion,
			Kind:       resourceKind,
			Name:       resourceName,
			Namespace:  testNS,
		},
		wantSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := fakedynamicclient.With(context.Background(), scheme.Scheme, tc.objects...)
			ctx = resource.WithDuck(ctx)
			r := NewSubjectResolver(ctx, func(types.NamespacedName) {})
			gotSelector, err := r.ResolveFromRef(tc.subject, genParent())
			if (err != nil) != tc.wantErr {
				t.Errorf("SubjectResolver.ResolveFromRef got err=%v want err=%v", err, tc.wantErr)
			}
			if diff := cmp.Diff(tc.wantSelector, gotSelector); diff != "" {
				t.Errorf("SubjectResolver.ResolveFromRef selector (-want,+got): %v", diff)
			}
		})
	}
}

func genParent() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceAPIVersion,
			"kind":       resourceKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      parentName,
			},
		},
	}
}

func genObject(annotation string, labels map[string]interface{}) *unstructured.Unstructured {
	var anno map[string]interface{}
	if annotation != "" {
		anno = map[string]interface{}{
			policy.AuthorizableAnnotationKey: annotation,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resourceAPIVersion,
			"kind":       resourceKind,
			"metadata": map[string]interface{}{
				"namespace":   testNS,
				"name":        resourceName,
				"annotations": anno,
				"labels":      labels,
			},
		},
	}
}
