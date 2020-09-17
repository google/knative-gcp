/*
Copyright 2019 Google LLC.

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

package v1beta1

import (
	"context"
	"testing"

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestPubSubSpec_SetPubSubDefaults(t *testing.T) {
	testCases := map[string]struct {
		orig     *PubSubSpec
		expected *PubSubSpec
		ctx      context.Context
	}{
		"missing defaults": {
			orig: &PubSubSpec{},
			expected: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "google-cloud-key",
					},
					Key: "key.json",
				},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"missing default GCP Auth ctx": {
			orig:     &PubSubSpec{},
			expected: &PubSubSpec{},
			ctx:      context.Background(),
		},
		"empty defaults": {
			orig: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{},
			},
			expected: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "google-cloud-key",
					},
					Key: "key.json",
				},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"secret exists same key": {
			orig: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "different-name",
					},
					Key: "key.json",
				},
			},
			expected: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "different-name",
					},
					Key: "key.json",
				},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"secret exists same name": {
			orig: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "google-cloud-key",
					},
					Key: "different-key.json",
				},
			},
			expected: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "google-cloud-key",
					},
					Key: "different-key.json",
				},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"secret exists all different": {
			orig: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "different-name",
					},
					Key: "different-key.json",
				},
			},
			expected: &PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "different-name",
					},
					Key: "different-key.json",
				},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"empty secret and non-empty serviceAccountName": {
			orig: &PubSubSpec{
				Secret:       &corev1.SecretKeySelector{},
				IdentitySpec: IdentitySpec{ServiceAccountName: "k8sServiceAccount"},
			},
			expected: &PubSubSpec{IdentitySpec: IdentitySpec{
				ServiceAccountName: "k8sServiceAccount"},
			},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.orig.SetPubSubDefaults(tc.ctx)
			if diff := cmp.Diff(tc.expected, tc.orig); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}
