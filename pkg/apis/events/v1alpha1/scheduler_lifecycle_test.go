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

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

func TestSchedulerStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *SchedulerStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &SchedulerStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			return s
		}(),
	}, {
		name: "topic not ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkPullSubscriptionReady()
			s.MarkJobReady("jobName")
			s.MarkTopicFailed("NotReady", "topic not ready")
			return s
		}(),
	}, {
		name: "pullsubscription not ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkTopicReady("topicID", "projectID")
			s.MarkPullSubscriptionFailed("NotReady", "ps not ready")
			s.MarkJobReady("jobName")
			return s
		}(),
	}, {
		name: "job not ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkTopicReady("topicID", "projectID")
			s.MarkPullSubscriptionReady()
			s.MarkJobNotReady("NotReady", "ps not ready")
			return s
		}(),
	}, {
		name: "ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkTopicReady("topicID", "projectID")
			s.MarkPullSubscriptionReady()
			s.MarkJobReady("jobName")
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

func TestSchedulerStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *SchedulerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &SchedulerStatus{},
		condQuery: SchedulerConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: SchedulerConditionReady,
		want: &apis.Condition{
			Type:   SchedulerConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkJobNotReady("NotReady", "test message")
			return s
		}(),
		condQuery: JobReady,
		want: &apis.Condition{
			Type:    JobReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *SchedulerStatus {
			s := &SchedulerStatus{}
			s.InitializeConditions()
			s.MarkJobReady("jobName")
			return s
		}(),
		condQuery: JobReady,
		want: &apis.Condition{
			Type:   JobReady,
			Status: corev1.ConditionTrue,
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
