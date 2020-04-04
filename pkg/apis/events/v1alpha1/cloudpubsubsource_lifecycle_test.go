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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestCloudPubSubSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *CloudPubSubSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{
		{
			name: "uninitialized",
			s:    &CloudPubSubSourceStatus{},
			want: false,
		}, {
			name: "initialized",
			s: func() *CloudPubSubSourceStatus {
				s := &CloudPubSubSource{}
				s.Status.InitializeConditions()
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		},
		{
			name: "the status of pullsubscription is false",
			s: func() *CloudPubSubSourceStatus {
				s := &CloudPubSubSource{}
				s.Status.InitializeConditions()
				s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "PullSubscriptionFalse", "status false test message")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionFalse,
		}, {
			name: "the status of pullsubscription is unknown",
			s: func() *CloudPubSubSourceStatus {
				s := &CloudPubSubSource{}
				s.Status.InitializeConditions()
				s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), "PullSubscriptionUnknonw", "status unknown test message")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
		},
		{
			name: "ready",
			s: func() *CloudPubSubSourceStatus {
				s := &CloudPubSubSource{}
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
func TestCloudPubSubSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *CloudPubSubSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &CloudPubSubSourceStatus{},
		condQuery: CloudPubSubSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *CloudPubSubSourceStatus {
			s := &CloudPubSubSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: CloudPubSubSourceConditionReady,
		want: &apis.Condition{
			Type:   CloudPubSubSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *CloudPubSubSourceStatus {
			s := &CloudPubSubSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "NotReady", "test message")
			return &s.Status
		}(),
		condQuery: duckv1alpha1.PullSubscriptionReady,
		want: &apis.Condition{
			Type:    duckv1alpha1.PullSubscriptionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *CloudPubSubSourceStatus {
			s := &CloudPubSubSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			return &s.Status
		}(),
		condQuery: duckv1alpha1.PullSubscriptionReady,
		want: &apis.Condition{
			Type:   duckv1alpha1.PullSubscriptionReady,
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
