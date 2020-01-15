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
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPubSubStatusIsReady(t *testing.T) {
	tests := []struct {
		name                   string
		pullsubscriptionStatus *pubsubv1alpha1.PullSubscriptionStatus
		wantConditionStatus    corev1.ConditionStatus
		want                   bool
	}{
		{
			name:                   "the status of pullsubscription is false",
			pullsubscriptionStatus: TestHelper.FalsePullSubscriptionStatus(),
			wantConditionStatus:    corev1.ConditionFalse,
		}, {
			name:                   "the status of pullsubscription is unknown",
			pullsubscriptionStatus: TestHelper.UnknownPullSubscriptionStatus(),
			wantConditionStatus:    corev1.ConditionUnknown,
		},
		{
			name:                   "ready",
			pullsubscriptionStatus: TestHelper.ReadyPullSubscriptionStatus(),
			wantConditionStatus:    corev1.ConditionTrue,
			want:                   true,
		}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := &PubSubStatus{}
			ps.PropagatePullSubscriptionStatus(test.pullsubscriptionStatus)
			gotConditionStatus := ps.GetTopLevelCondition().Status
			got := ps.IsReady()
			if gotConditionStatus != test.wantConditionStatus {
				t.Errorf("unexpected condition status: want %v, got %v", test.wantConditionStatus, gotConditionStatus)
			}
			if got != test.want {
				t.Errorf("unexpected readiniess: want %v, got %v", test.want, got)
			}
		})
	}
}
func TestPubSubStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *PubSubStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &PubSubStatus{},
		condQuery: PubSubConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: PubSubConditionReady,
		want: &apis.Condition{
			Type:   PubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.InitializeConditions()
			s.MarkPullSubscriptionFailed("NotReady", "test message")
			return s
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
		s: func() *PubSubStatus {
			s := &PubSubStatus{}
			s.InitializeConditions()
			s.MarkPullSubscriptionReady()
			return s
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
