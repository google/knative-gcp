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
	"time"

	authorizationtesthelper "github.com/google/knative-gcp/pkg/apis/configs/authorization/testhelper"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCloudPubSubSourceDefaults(t *testing.T) {

	defaultRetentionDuration := defaultRetentionDuration
	defaultAckDeadline := defaultAckDeadline

	tests := []struct {
		name  string
		start *CloudPubSubSource
		want  *CloudPubSubSource
	}{{
		name: "non-nil",
		start: &CloudPubSubSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-cloud-key",
						},
						Key: "test.json",
					},
				},
			},
		},
		want: &CloudPubSubSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1alpha1.PubSubSpec{
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
		name: "nil, except cluster name annotations",
		start: &CloudPubSubSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudPubSubSourceSpec{},
		},
		want: &CloudPubSubSource{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1alpha1.PubSubSpec{
					Secret: &authorizationtesthelper.Secret,
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.start
			got.SetDefaults(authorizationtesthelper.ContextWithDefaults())

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}

func TestCloudPubSubSourceDefaults_NoChange(t *testing.T) {
	days2 := 2 * 24 * time.Hour
	secs60 := 60 * time.Second
	want := &CloudPubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				duckv1alpha1.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
			},
		},
		Spec: CloudPubSubSourceSpec{
			AckDeadline:       ptr.String(secs60.String()),
			RetentionDuration: ptr.String(days2.String()),
			PubSubSpec: duckv1alpha1.PubSubSpec{
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
	got.SetDefaults(context.Background())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
