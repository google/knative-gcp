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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPolicyBindingStatusIsReady(t *testing.T) {
	cases := []struct {
		name                string
		s                   *PolicyBindingStatus
		wantConditionStatus corev1.ConditionStatus
		wantReady           bool
	}{{
		name:      "uninitialized",
		s:         &PolicyBindingStatus{},
		wantReady: false,
	}, {
		name: "initialized",
		s: func() *PolicyBindingStatus {
			s := &PolicyBindingStatus{}
			s.InitializeConditions()
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		wantReady:           false,
	}, {
		name: "binding failure",
		s: func() *PolicyBindingStatus {
			s := &PolicyBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingFailure("BindingFailure", "failure")
			return s
		}(),
		wantConditionStatus: corev1.ConditionFalse,
		wantReady:           false,
	}, {
		name: "binding ready",
		s: func() *PolicyBindingStatus {
			s := &PolicyBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingAvailable()
			return s
		}(),
		wantConditionStatus: corev1.ConditionTrue,
		wantReady:           true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantConditionStatus != "" {
				gotConditionStatus := tc.s.GetTopLevelCondition().Status
				if gotConditionStatus != tc.wantConditionStatus {
					t.Errorf("unexpected condition status: want %v, got %v", tc.wantConditionStatus, gotConditionStatus)
				}
			}
			got := tc.s.IsReady()
			if got != tc.wantReady {
				t.Errorf("unexpected readiness: want %v, got %v", tc.wantReady, got)
			}
		})
	}
}

func TestPropagateBindingStatus(t *testing.T) {
	cases := []struct {
		name  string
		other *PolicyBindingStatus
	}{{
		name: "other is initialized",
		other: func() *PolicyBindingStatus {
			p := &PolicyBindingStatus{}
			p.InitializeConditions()
			return p
		}(),
	}, {
		name: "other is ready",
		other: func() *PolicyBindingStatus {
			p := &PolicyBindingStatus{}
			p.InitializeConditions()
			p.MarkBindingAvailable()
			return p
		}(),
	}, {
		name: "other is failure",
		other: func() *PolicyBindingStatus {
			p := &PolicyBindingStatus{}
			p.InitializeConditions()
			p.MarkBindingFailure("FailureReason", "failure message")
			return p
		}(),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &PolicyBindingStatus{}
			s.InitializeConditions()
			s.PropagateBindingStatus(tc.other)
			wantCond := tc.other.GetTopLevelCondition()
			gotCond := s.GetTopLevelCondition()
			if wantCond == nil && gotCond != nil {
				t.Errorf("propagated status top level condition got=%v, want=nil", gotCond)
				return
			}
			if diff := cmp.Diff(wantCond, gotCond, cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
				t.Errorf("propagated status top level condition (-want,+got): %v", diff)
			}
		})
	}
}
