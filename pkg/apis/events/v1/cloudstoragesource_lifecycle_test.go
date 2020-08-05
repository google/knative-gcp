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

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

func TestCloudStorageSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *CloudStorageSourceStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &CloudStorageSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "the status of topic is false",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkNotificationReady("notificationID")
			s.Status.MarkTopicFailed(s.ConditionSet(), "TopicFailed", "the status of topic is false")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "the status of topic is unknown",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkNotificationReady("notificationID")
			s.Status.MarkTopicUnknown(s.ConditionSet(), "TopicUnknown", "the status of topic is unknown")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "the status of pullsubscription is false",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkTopicReady(s.ConditionSet())
			s.Status.MarkPullSubscriptionFailed(s.ConditionSet(), "PullSubscriptionFailed", "the status of pullsubscription is false")
			s.Status.MarkNotificationReady("notificationID")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "the status of pullsubscription is unknown",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkTopicReady(s.ConditionSet())
			s.Status.MarkPullSubscriptionUnknown(s.ConditionSet(), "PullSubscriptionUnknown", "the status of pullsubscription is unknown")
			s.Status.MarkNotificationReady("notificationID")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "notification not ready",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkTopicReady(s.ConditionSet())
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkNotificationNotReady("NotReady", "notification not ready")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name: "the status of notification is unknown",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkTopicReady(s.ConditionSet())
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkNotificationUnknown("Unknown", "notification not ready")
			return &s.Status
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "ready",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSource{}
			s.Status.InitializeConditions()
			s.Status.MarkTopicReady(s.ConditionSet())
			s.Status.MarkPullSubscriptionReady(s.ConditionSet())
			s.Status.MarkNotificationReady("notificationID")
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

func TestCloudStorageSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *CloudStorageSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &CloudStorageSourceStatus{},
		condQuery: CloudStorageSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: CloudStorageSourceConditionReady,
		want: &apis.Condition{
			Type:   CloudStorageSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSourceStatus{}
			s.InitializeConditions()
			s.MarkNotificationNotReady("NotReady", "test message")
			return s
		}(),
		condQuery: NotificationReady,
		want: &apis.Condition{
			Type:    NotificationReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "unknown",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSourceStatus{}
			s.InitializeConditions()
			s.MarkNotificationUnknown("Unknown", "test message")
			return s
		}(),
		condQuery: NotificationReady,
		want: &apis.Condition{
			Type:    NotificationReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Unknown",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *CloudStorageSourceStatus {
			s := &CloudStorageSourceStatus{}
			s.InitializeConditions()
			s.MarkNotificationReady("notificationID")
			return s
		}(),
		condQuery: NotificationReady,
		want: &apis.Condition{
			Type:   NotificationReady,
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
