/*
Copyright 2019 The Knative Authors

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

	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

func TestTopicDefaults(t *testing.T) {
	testCases := map[string]struct {
		want *Topic
		got  *Topic
		ctx  context.Context
	}{
		"with GCP Auth": {
			want: &Topic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: TopicSpec{
					PropagationPolicy: TopicPolicyCreateNoDelete,
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "google-cloud-key",
						},
						Key: "key.json",
					},
					EnablePublisher: &trueVal,
				}},
			got: &Topic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: TopicSpec{}},
			ctx: gcpauthtesthelper.ContextWithDefaults(),
		},
		"without GCP Auth": {
			want: &Topic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
				Spec: TopicSpec{
					PropagationPolicy: TopicPolicyCreateNoDelete},
			},
			got: &Topic{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
					},
				},
			},
			ctx: context.Background(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.got.SetDefaults(tc.ctx)
			if diff := cmp.Diff(tc.want, tc.got); diff != "" {
				t.Errorf("Unexpected differences (-want +got): %v", diff)
			}
		})
	}
}
