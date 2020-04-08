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

package v1alpha1

import (
	"context"
	"testing"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSourceDefaults(t *testing.T) {
	defaultTopic := DefaultTopic
	tests := []struct {
		name  string
		start *CloudBuildSource
		want  *CloudBuildSource
	}{{
		name: "defaults present",
		start: &CloudBuildSource{
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cloud-key",
						},
						Key: "test.json",
					},
				},
				Topic: ptr.String(defaultTopic),
			},
		},
		want: &CloudBuildSource{
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cloud-key",
						},
						Key: "test.json",
					},
				},
				Topic: ptr.String(defaultTopic),
			},
		},
	}, {
		name: "missing defaults",
		start: &CloudBuildSource{
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       CloudBuildSourceSpec{},
		},
		want: &CloudBuildSource{
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: duckv1alpha1.DefaultGoogleCloudSecretSelector(),
				},
				Topic: ptr.String(defaultTopic),
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.start
			got.SetDefaults(context.Background())

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}

func TestCloudBuildSourceDefaults_NoChange(t *testing.T) {
	want := &CloudBuildSource{
		Spec: CloudBuildSourceSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-cloud-key",
					},
					Key: "test.json",
				},
			},
			Topic: ptr.String(DefaultTopic),
		},
	}

	got := want.DeepCopy()
	got.SetDefaults(context.Background())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
