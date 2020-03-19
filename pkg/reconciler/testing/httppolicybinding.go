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

package testing

import (
	"context"

	"github.com/google/knative-gcp/pkg/apis/security/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

type BindingOption func(*v1alpha1.HTTPPolicyBinding)

func NewPolicyBinding(name, namespace string, opts ...BindingOption) *v1alpha1.HTTPPolicyBinding {
	b := &v1alpha1.HTTPPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid",
		},
	}
	for _, opt := range opts {
		opt(b)
	}
	b.SetDefaults(context.Background())
	return b
}

func WithPolicyBindingSubject(gvk metav1.GroupVersionKind, name string) BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Subject = tracker.Reference{
			APIVersion: apiVersion(gvk),
			Kind:       gvk.Kind,
			Name:       name,
			Namespace:  b.Namespace,
		}
	}
}

func WithPolicyBindingSubjectLabels(labels map[string]string) BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Subject = tracker.Reference{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		}
	}
}

func WithPolicyBindingPolicy(name string) BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Spec.Policy = duckv1.KReference{Name: name}
	}
}

func WithPolicyBindingStatusInit() BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
	}
}

func WithPolicyBindingStatusReady() BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
		b.Status.MarkBindingAvailable()
	}
}

func WithPolicyBindingStatusFailure(reason, message string) BindingOption {
	return func(b *v1alpha1.HTTPPolicyBinding) {
		b.Status.InitializeConditions()
		b.Status.MarkBindingFailure(reason, message)
	}
}
