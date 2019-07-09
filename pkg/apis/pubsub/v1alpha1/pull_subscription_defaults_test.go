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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPullSubscriptionDefaults(t *testing.T) {
	tests := []struct {
		name  string
		start *PullSubscription
		want  *PullSubscription
	}{{
		name: "non-nil structured",
		start: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode: ModeCloudEventsStructured,
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-cloud-key",
					},
					Key: "test.json",
				},
			},
		},
		want: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode: ModeCloudEventsStructured,
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-cloud-key",
					},
					Key: "test.json",
				},
			},
		},
	}, {
		name: "non-nil push",
		start: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode: ModePushCompatible,
			},
		},
		want: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode:   ModePushCompatible,
				Secret: defaultSecretSelector(),
			},
		},
	}, {
		name: "non-nil invalid",
		start: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode: "invalid",
			},
		},
		want: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode:   ModeCloudEventsBinary,
				Secret: defaultSecretSelector(),
			},
		},
	}, {
		name: "nil",
		start: &PullSubscription{
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       PullSubscriptionSpec{},
		},
		want: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode:   ModeCloudEventsBinary,
				Secret: defaultSecretSelector(),
			},
		},
	}, {
		name: "nil secret",
		start: &PullSubscription{
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       PullSubscriptionSpec{},
		},
		want: &PullSubscription{
			Spec: PullSubscriptionSpec{
				Mode:   ModeCloudEventsBinary,
				Secret: defaultSecretSelector(),
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
