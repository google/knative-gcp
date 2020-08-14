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

package customresource

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/broker/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	apis "knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestRetrieveLatestUpdateTime(t *testing.T) {
	anchorTime := time.Now()
	creationTime := anchorTime.Add(10 * time.Minute)
	trigger2StatusUpdate1 := anchorTime.Add(20 * time.Minute)
	trigger2StatusUpdate2 := anchorTime.Add(30 * time.Minute)
	deletionTime := anchorTime.Add(100 * time.Minute)

	tests := []struct {
		name     string
		resource *v1beta1.Trigger
		expected time.Time
	}{
		{
			name: "Creation time is selected when there are no other updates",
			resource: &v1beta1.Trigger{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{
						Time: creationTime,
					},
				},
				Status: v1beta1.TriggerStatus{},
			},
			expected: creationTime,
		},
		{
			name: "Latest status update is selected",
			resource: &v1beta1.Trigger{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{
						Time: trigger2StatusUpdate1,
					},
				},
				Status: v1beta1.TriggerStatus{
					TriggerStatus: eventingv1beta1.TriggerStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{
								{
									LastTransitionTime: apis.VolatileTime{
										Inner: metav1.Time{
											Time: trigger2StatusUpdate1,
										},
									},
								},
								{
									LastTransitionTime: apis.VolatileTime{
										Inner: metav1.Time{
											Time: trigger2StatusUpdate2,
										},
									},
								},
							},
						},
					},
				},
			},
			expected: trigger2StatusUpdate2,
		},
		{
			name: "Deletion time is picked as the latest",
			resource: &v1beta1.Trigger{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{
						Time: trigger2StatusUpdate1,
					},
					DeletionTimestamp: &metav1.Time{
						Time: deletionTime,
					},
				},
				Status: v1beta1.TriggerStatus{
					TriggerStatus: eventingv1beta1.TriggerStatus{
						Status: duckv1.Status{
							Conditions: []apis.Condition{
								{
									LastTransitionTime: apis.VolatileTime{
										Inner: metav1.Time{
											Time: trigger2StatusUpdate1,
										},
									},
								},
								{
									LastTransitionTime: apis.VolatileTime{
										Inner: metav1.Time{
											Time: trigger2StatusUpdate2,
										},
									},
								},
							},
						},
					},
				},
			},
			expected: deletionTime,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := RetrieveLatestUpdateTime(test.resource)

			if err != nil {
				t.Errorf("An error: %w", err)
			}

			if diff := cmp.Diff(test.expected, result); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}
