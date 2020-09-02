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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	"github.com/google/knative-gcp/pkg/apis/duck"
	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSourceDefaults(t *testing.T) {
	tests := []struct {
		name  string
		start *CloudBuildSource
		want  *CloudBuildSource
	}{{
		name: "defaults present",
		start: &CloudBuildSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cloud-key",
						},
						Key: "test.json",
					},
				},
			},
		},
		want: &CloudBuildSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cloud-key",
						},
						Key: "test.json",
					},
				},
			},
		},
	}, {
		// Due to the limitation mentioned in https://github.com/google/knative-gcp/issues/1037, specifying the cluster name annotation.
		name: "missing defaults, except cluster name annotations",
		start: &CloudBuildSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudBuildSourceSpec{},
		},
		want: &CloudBuildSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudBuildSourceSpec{
				PubSubSpec: duckv1.PubSubSpec{
					Secret: &gcpauthtesthelper.Secret,
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.start
			got.SetDefaults(gcpauthtesthelper.ContextWithDefaults())

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}

func TestCloudBuildSourceDefaults_NoChange(t *testing.T) {
	want := &CloudBuildSource{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
			},
		},
		Spec: CloudBuildSourceSpec{
			PubSubSpec: duckv1.PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-cloud-key",
					},
					Key: "test.json",
				},
			},
		},
	}

	got := want.DeepCopy()
	got.SetDefaults(gcpauthtesthelper.ContextWithDefaults())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
