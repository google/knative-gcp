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

package v1beta1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/intevents"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestCloudPubSubSourceDefaults(t *testing.T) {

	defaultRetentionDuration := intevents.DefaultRetentionDuration
	defaultAckDeadline := intevents.DefaultAckDeadline

	tests := []struct {
		name  string
		start *CloudPubSubSource
		want  *CloudPubSubSource
	}{{
		name: "non-nil",
		start: &CloudPubSubSource{
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1beta1.PubSubSpec{
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
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1beta1.PubSubSpec{
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
		name: "nil",
		start: &CloudPubSubSource{
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       CloudPubSubSourceSpec{},
		},
		want: &CloudPubSubSource{
			Spec: CloudPubSubSourceSpec{
				RetentionDuration: ptr.String(defaultRetentionDuration.String()),
				AckDeadline:       ptr.String(defaultAckDeadline.String()),
				PubSubSpec: duckv1beta1.PubSubSpec{
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

func TestCloudPubSubSourceDefaults_NoChange(t *testing.T) {
	days2 := 2 * 24 * time.Hour
	secs60 := 60 * time.Second
	want := &CloudPubSubSource{
		Spec: CloudPubSubSourceSpec{
			AckDeadline:       ptr.String(secs60.String()),
			RetentionDuration: ptr.String(days2.String()),
			PubSubSpec: duckv1beta1.PubSubSpec{
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
