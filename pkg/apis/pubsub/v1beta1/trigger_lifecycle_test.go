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

package v1beta1

import (
	"testing"

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

func TestTriggerStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *TriggerStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name: "uninitialized",
		s:    &TriggerStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *TriggerStatus {
			s := &TriggerStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "trigger not ready",
		s: func() *TriggerStatus {
			s := &Trigger{}
			s.Status.InitializeConditions()
			s.Status.MarkTriggerNotReady("NotReady", "trigger not ready")
			return &s.Status
		}(),
	}, {
		name: "ready",
		s: func() *TriggerStatus {
			s := &Trigger{}
			s.Status.InitializeConditions()
			s.Status.MarkTriggerReady("triggerID")
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

func TestTriggerStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *TriggerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &TriggerStatus{},
		condQuery: TriggerConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *TriggerStatus {
			s := &TriggerStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: TriggerConditionReady,
		want: &apis.Condition{
			Type:   TriggerConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "not ready",
		s: func() *TriggerStatus {
			s := &TriggerStatus{}
			s.InitializeConditions()
			s.MarkTriggerNotReady("NotReady", "test message")
			return s
		}(),
		condQuery: TriggerReady,
		want: &apis.Condition{
			Type:    TriggerReady,
			Status:  corev1.ConditionFalse,
			Reason:  "NotReady",
			Message: "test message",
		},
	}, {
		name: "ready",
		s: func() *TriggerStatus {
			s := &TriggerStatus{}
			s.InitializeConditions()
			s.MarkTriggerReady("triggerID")
			return s
		}(),
		condQuery: TriggerReady,
		want: &apis.Condition{
			Type:   TriggerReady,
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
