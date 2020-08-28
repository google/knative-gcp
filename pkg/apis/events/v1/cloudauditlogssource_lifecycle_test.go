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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestCloudAuditLogsSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *CloudAuditLogsSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &CloudAuditLogsSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "the status of topic is false",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkSinkReady()
			s.Status.MarkTopicFailed(s.ConditionSet(), "test", "the status of topic is false")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "the status of topic is unknown",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkSinkReady()
			s.Status.MarkTopicUnknown(s.ConditionSet(), "test", "the status of topic is unknown")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	},
		{
			name: "the status of pullsubscription is false",
			s: func() *CloudAuditLogsSourceStatus {
				s := &CloudAuditLogsSource{}
				s.Status.InitializeConditions()
				s.Status.MarkTopicReady(s.ConditionSet())
				s.Status.MarkSinkReady()
				s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "test", "the status of pullsubscription is false")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionFalse,
		}, {
			name: "the status of pullsubscription is unknown",
			s: func() *CloudAuditLogsSourceStatus {
				s := &CloudAuditLogsSource{}
				s.Status.InitializeConditions()
				s.Status.MarkTopicReady(s.ConditionSet())
				s.Status.MarkSinkReady()
				s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), "test", "the status of pullsubscription is unknown")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		}, {
			name: "sink is not ready",
			s: func() *CloudAuditLogsSourceStatus {
				s := &CloudAuditLogsSource{}
				s.Status.InitializeConditions()
				s.Status.MarkTopicReady(s.ConditionSet())
				s.Status.MarkPullSubscriptionReady(s.ConditionSet())
				s.Status.MarkSinkNotReady("test", "sink is not ready")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionFalse,
			want:                false,
		}, {
			name: "the status of sink is unknown",
			s: func() *CloudAuditLogsSourceStatus {
				s := &CloudAuditLogsSource{}
				s.Status.InitializeConditions()
				s.Status.MarkTopicReady(s.ConditionSet())
				s.Status.MarkPullSubscriptionReady(s.ConditionSet())
				s.Status.MarkSinkUnknown("test", "the status of sink is unknown")
				return &s.Status
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		}, {
			name: "ready",
			s: func() *CloudAuditLogsSourceStatus {
				s := &CloudAuditLogsSource{}
				s.Status.InitializeConditions()
				s.Status.MarkTopicReady(s.ConditionSet())
				s.Status.MarkPullSubscriptionReady(s.ConditionSet())
				s.Status.MarkSinkReady()
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
func TestCloudAuditLogsSourceGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *CloudAuditLogsSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &CloudAuditLogsSourceStatus{},
		condQuery: SinkReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: SinkReady,
		want: &apis.Condition{
			Type:   SinkReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSourceStatus{}
			s.InitializeConditions()
			s.MarkSinkNotReady("NotReady", "test message")
			return s
		}(),
		condQuery: SinkReady,
		want: &apis.Condition{
			Type:    SinkReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "unknown",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSourceStatus{}
			s.InitializeConditions()
			s.MarkSinkUnknown("Unknown", "test message")
			return s
		}(),
		condQuery: SinkReady,
		want: &apis.Condition{
			Type:    SinkReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Unknown",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *CloudAuditLogsSourceStatus {
			s := &CloudAuditLogsSourceStatus{}
			s.InitializeConditions()
			s.MarkSinkReady()
			return s
		}(),
		condQuery: SinkReady,
		want: &apis.Condition{
			Type:   SinkReady,
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
