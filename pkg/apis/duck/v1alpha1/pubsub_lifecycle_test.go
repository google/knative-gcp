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

package v1alpha1

import (
	"testing"

	"knative.dev/pkg/apis"
)

var cs = apis.NewLivingConditionSet(
	PullSubscriptionReady,
	TopicReady,
)

func TestPubSubStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		ps   *PubSubStatus
		want bool
	}{{
		name: "uninitialized",
		ps:   &PubSubStatus{},
		want: false,
	}, {
		name: "initialized",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			return s
		}(),
		want: false,
	}, {
		name: "topic is not configured",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionReady(&cs)
			s.MarkTopicNotConfigured(&cs)
			return s
		}(),
		want: false,
	}, {
		name: "the status of topic is false",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionReady(&cs)
			s.MarkTopicFailed(&cs, "test", "the status of topic is false")
			return s
		}(),
		want: false,
	}, {
		name: "the status of topic is unknown",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionReady(&cs)
			s.MarkTopicUnknown(&cs, "test", "the status of topic is unknown")
			return s
		}(),
		want: false,
	}, {
		name: "pullsubscription is not configured",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionNotConfigured(&cs)
			s.MarkTopicReady(&cs)
			return s
		}(),
		want: false,
	}, {
		name: "the status of pullsubscription is false",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionFailed(&cs, "test", "the status of pullsubscription is false")
			s.MarkTopicReady(&cs)
			return s
		}(),
		want: false,
	}, {
		name: "the status pullsubscription is unknown",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionUnknown(&cs, "test", "the status of pullsubscription is unknown")
			s.MarkTopicReady(&cs)
			return s
		}(),
		want: false,
	}, {
		name: "mark ready",
		ps: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.MarkPullSubscriptionReady(&cs)
			s.MarkTopicReady(&cs)
			return s
		}(),
		want: true,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ps.IsReady()
			if got != test.want {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}
