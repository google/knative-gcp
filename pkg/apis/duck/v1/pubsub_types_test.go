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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestPubSub_GetFullType(t *testing.T) {
	ps := &PubSub{}
	switch ps.GetFullType().(type) {
	case *PubSub:
		// expected
	default:
		t.Errorf("expected GetFullType to return *PubSub, got %T", ps.GetFullType())
	}
}

func TestPubSub_GetListType(t *testing.T) {
	ps := &PubSub{}
	switch ps.GetListType().(type) {
	case *PubSubList:
		// expected
	default:
		t.Errorf("expected GetListType to return *PubSubList, got %T", ps.GetListType())
	}
}

func TestPubSub_Populate(t *testing.T) {
	got := &PubSub{}

	want := &PubSub{
		Spec: PubSubSpec{
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					URI: &apis.URL{
						Scheme:   "https",
						Host:     "tableflip.dev",
						RawQuery: "flip=mattmoor",
					},
				},
				CloudEventOverrides: &duckv1.CloudEventOverrides{
					Extensions: map[string]string{
						"boosh": "kakow",
					},
				},
			},
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
				Key:                  "secretkey",
			},
		},

		Status: PubSubStatus{
			IdentityStatus: IdentityStatus{
				Status: duckv1.Status{
					ObservedGeneration: 42,
					Conditions: duckv1.Conditions{
						{
							// Populate ALL fields
							Type:               duckv1.SourceConditionSinkProvided,
							Status:             corev1.ConditionTrue,
							LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Date(1984, 02, 28, 18, 52, 00, 00, time.UTC))},
						},
					},
				},
			},
			SinkURI: &apis.URL{
				Scheme:   "https",
				Host:     "tableflip.dev",
				RawQuery: "flip=mattmoor",
			},
			ProjectID:      "projectid",
			TopicID:        "topicid",
			SubscriptionID: "subscriptionid",
		},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}
