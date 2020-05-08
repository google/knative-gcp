/*
Copyright 2019 Google LLC

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

	"github.com/google/go-cmp/cmp"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCloudStorageSourceSpec_SetDefaults(t *testing.T) {
	testCases := map[string]struct {
		orig     *CloudStorageSourceSpec
		expected *CloudStorageSourceSpec
	}{
		"missing defaults": {
			orig: &CloudStorageSourceSpec{},
			expected: &CloudStorageSourceSpec{
				EventTypes: allEventTypes,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "google-cloud-key",
						},
						Key: "key.json",
					},
				},
			},
		},
		"defaults present": {
			orig: &CloudStorageSourceSpec{
				EventTypes: []string{CloudStorageSourceFinalize, CloudStorageSourceDelete},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret-name",
						},
						Key: "secret-key.json",
					},
				},
			},
			expected: &CloudStorageSourceSpec{
				EventTypes: []string{CloudStorageSourceFinalize, CloudStorageSourceDelete},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret-name",
						},
						Key: "secret-key.json",
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.orig.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.orig); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}

func TestCloudStorageSource_SetDefaults(t *testing.T) {
	testCases := map[string]struct {
		orig     *CloudStorageSource
		expected *CloudStorageSource
	}{
		"missing defaults, except cluster name annotations": {
			orig: &CloudStorageSource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
			},
			expected: &CloudStorageSource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: CloudStorageSourceSpec{
					EventTypes: []string{
						"com.google.cloud.storage.object.finalize",
						"com.google.cloud.storage.object.delete",
						"com.google.cloud.storage.object.archive",
						"com.google.cloud.storage.object.metadataUpdate",
					},
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "google-cloud-key",
							},
							Key: "key.json",
						},
					},
				},
			},
		},
		"defaults present": {
			orig: &CloudStorageSource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: CloudStorageSourceSpec{
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret-name",
							},
							Key: "secret-key.json",
						},
					},
				},
			},
			expected: &CloudStorageSource{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: CloudStorageSourceSpec{
					EventTypes: []string{
						"com.google.cloud.storage.object.finalize",
						"com.google.cloud.storage.object.delete",
						"com.google.cloud.storage.object.archive",
						"com.google.cloud.storage.object.metadataUpdate",
					},
					PubSubSpec: duckv1alpha1.PubSubSpec{
						Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret-name",
							},
							Key: "secret-key.json",
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.orig.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.orig); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}
