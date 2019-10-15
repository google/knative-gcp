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
	corev1 "k8s.io/api/core/v1"
)

func TestStorage_SetDefaults(t *testing.T) {
	testCases := map[string]struct {
		orig     *StorageSpec
		expected *StorageSpec
	}{
		"missing defaults": {
			orig: &StorageSpec{},
			expected: &StorageSpec{
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
		"empty defaults": {
			orig: &StorageSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{},
				},
			},
			expected: &StorageSpec{
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
		"secret exists same key": {
			orig: &StorageSpec{
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "different-name",
						},
						Key: "key.json",
					},
				},
			},
			expected: &StorageSpec{
				EventTypes: allEventTypes,
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "different-name",
						},
						Key: "key.json",
					},
				},
			},
		},
		"secret exists same name": {
			orig: &StorageSpec{
				EventTypes: []string{StorageFinalize},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "google-cloud-key",
						},
						Key: "different-key.json",
					},
				},
			},
			expected: &StorageSpec{
				EventTypes: []string{StorageFinalize},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "google-cloud-key",
						},
						Key: "different-key.json",
					},
				},
			},
		},
		"secret exists all different": {
			orig: &StorageSpec{
				EventTypes: []string{StorageFinalize, StorageDelete},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "different-name",
						},
						Key: "different-key.json",
					},
				},
			},
			expected: &StorageSpec{
				EventTypes: []string{StorageFinalize, StorageDelete},
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "different-name",
						},
						Key: "different-key.json",
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
