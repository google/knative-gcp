/*
Copyright 2020 Google LLC

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

	"github.com/google/go-cmp/cmp"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestTriggerDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Trigger
		expected Trigger
	}{
		"non-knative v1alpha1 service subscriber": {
			initial: Trigger{
				Spec: eventingv1beta1.TriggerSpec{
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1",
							Kind:       "Service",
						},
					},
				}},
			expected: Trigger{
				Spec: eventingv1beta1.TriggerSpec{
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1",
							Kind:       "Service",
						},
					}}},
		},
		"knative v1alpha1 service subscriber": {
			initial: Trigger{
				Spec: eventingv1beta1.TriggerSpec{
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1alpha1",
							Kind:       "Service",
						},
					},
				}},
			expected: Trigger{
				Spec: eventingv1beta1.TriggerSpec{
					Subscriber: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "serving.knative.dev/v1",
							Kind:       "Service",
						},
					}}},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
