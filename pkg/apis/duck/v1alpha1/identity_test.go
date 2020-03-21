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
	"reflect"
	"testing"
)

func TestSetWorkloadIdentityStatus(t *testing.T) {
	tests := []struct {
		name string
		wis  *WorkloadIdentityStatus
		want *WorkloadIdentityStatus
	}{{
		name: "all happy",
		wis:  &WorkloadIdentityStatus{},
		want: &WorkloadIdentityStatus{"True", "True", "", "", "test123"},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.wis.SetWorkloadIdentityStatus("True", "True", "", "", "test123")
			if got := test.wis; !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}

func TestInitWorkloadIdentityStatus(t *testing.T) {
	got := &WorkloadIdentityStatus{}
	got.InitWorkloadIdentityStatus()
	want := &WorkloadIdentityStatus{"False", "Unknown", "", "", ""}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestInitReconcileWorkloadIdentityStatus(t *testing.T) {
	got := &WorkloadIdentityStatus{}
	got.InitReconcileWorkloadIdentityStatus()
	want := &WorkloadIdentityStatus{"True", "False", WorkloadIdentityFailed, "", ""}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestInitDeleteWorkloadIdentityStatus(t *testing.T) {
	got := &WorkloadIdentityStatus{}
	got.InitDeleteWorkloadIdentityStatus()
	want := &WorkloadIdentityStatus{"True", "False", DeleteWorkloadIdentityFailed, "", ""}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestMarkWorkloadIdentityReconciled(t *testing.T) {
	got := &WorkloadIdentityStatus{}
	got.MarkWorkloadIdentityReconciled()
	want := &WorkloadIdentityStatus{"True", "True", WorkloadIdentitySucceed, "", ""}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestMarkWorkloadIdentityDeleted(t *testing.T) {
	got := &WorkloadIdentityStatus{}
	got.MarkWorkloadIdentityDeleted()
	want := &WorkloadIdentityStatus{"True", "True", DeleteWorkloadIdentitySucceed, "", ""}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}
