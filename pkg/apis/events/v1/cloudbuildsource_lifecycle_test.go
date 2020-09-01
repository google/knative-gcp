/*
Copyright 2020 Google LLC.

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

	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestCloudBuildSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *CloudBuildSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{
		{
			name: "uninitialized",
			s:    &CloudBuildSourceStatus{},
			want: false,
		}, {
			name: "initialized",
			s: func() *CloudBuildSourceStatus {
				s := &CloudBuildSource{}
				s.Status.InitializeConditions()
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		},
		{
			name: "the status of pullsubscription is false",
			s: func() *CloudBuildSourceStatus {
				s := &CloudBuildSource{}
				s.Status.InitializeConditions()
				s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "PullSubscriptionFalse", "status false test message")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionFalse,
		}, {
			name: "the status of pullsubscription is unknown",
			s: func() *CloudBuildSourceStatus {
				s := &CloudBuildSource{}
				s.Status.InitializeConditions()
				s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), "PullSubscriptionUnknown", "status unknown test message")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
		},
		{
			name: "ready",
			s: func() *CloudBuildSourceStatus {
				s := &CloudBuildSource{}
				s.Status.InitializeConditions()
				s.Status.MarkPullSubscriptionReady(s.ConditionSet())
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionTrue,
			want:                true,
		}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.wantConditionStatus != "" {
				gotConditionStatus := test.s.GetTopLevelCondition().Status
				if gotConditionStatus != test.wantConditionStatus {
					t.Errorf("unexpected condition status: want %v, got %v", test.wantConditionStatus, gotConditionStatus)
				}
			}
			got := test.s.IsReady()
			if got != test.want {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}
func TestCloudBuildSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *CloudBuildSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &CloudBuildSourceStatus{},
		condQuery: CloudBuildSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *CloudBuildSourceStatus {
			s := &CloudBuildSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: CloudBuildSourceConditionReady,
		want: &apis.Condition{
			Type:   CloudBuildSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",

		s: func() *CloudBuildSourceStatus {
			s := &CloudBuildSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "NotReady", "test message")
			return &s.Status
		}(),
		condQuery: duckv1.PullSubscriptionReady,
		want: &apis.Condition{
			Type:    duckv1.PullSubscriptionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *CloudBuildSourceStatus {
			s := &CloudBuildSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			return &s.Status
		}(),
		condQuery: duckv1.PullSubscriptionReady,
		want: &apis.Condition{
			Type:   duckv1.PullSubscriptionReady,
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
