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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
)

func TestTopicStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *TopicStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &TopicStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark deployed",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		want: false,
	}, {
		name: "mark addressable",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkTopicReady()
			s.MarkDeployed()
			s.SetAddress(&apis.URL{})
			return s
		}(),
		want: true,
	}, {
		name: "mark nil addressable",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkTopicReady()
			s.MarkDeployed()
			s.SetAddress(nil)
			return s
		}(),
		want: false,
	}, {
		name: "mark not deployed then deploying then deployed",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkTopicReady()
			s.SetAddress(&apis.URL{})
			s.MarkNotDeployed("MarkNotDeployed", "")
			s.MarkDeploying("MarkDeploying", "")
			s.MarkDeployed()
			return s
		}(),
		want: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestTopicStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *TopicStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &TopicStatus{},
		condQuery: TopicConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: TopicConditionReady,
		want: &apis.Condition{
			Type:   TopicConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		condQuery: TopicConditionReady,
		want: &apis.Condition{
			Type:   TopicConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark topic ready",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkTopicReady()
			return s
		}(),
		condQuery: TopicConditionTopicExists,
		want: &apis.Condition{
			Type:   TopicConditionTopicExists,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark no topic",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkNoTopic("reason", "%s", "message")
			return s
		}(),
		condQuery: TopicConditionTopicExists,
		want: &apis.Condition{
			Type:    TopicConditionTopicExists,
			Status:  corev1.ConditionFalse,
			Reason:  "reason",
			Message: "message",
		},
	}, {
		name: "mark topic operating",
		s: func() *TopicStatus {
			s := &TopicStatus{}
			s.InitializeConditions()
			s.MarkTopicOperating("reason", "%s", "message")
			return s
		}(),
		condQuery: TopicConditionTopicExists,
		want: &apis.Condition{
			Type:    TopicConditionTopicExists,
			Status:  corev1.ConditionUnknown,
			Reason:  "reason",
			Message: "message",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}
