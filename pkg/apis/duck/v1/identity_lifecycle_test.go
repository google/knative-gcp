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

	"knative.dev/pkg/apis"
)

func TestMarkWorkloadIdentityReady(t *testing.T) {
	status := &IdentityStatus{}
	condSet := apis.NewLivingConditionSet()
	status.MarkWorkloadIdentityReady(&condSet)
	got := status.IsReady()
	want := true
	if got != want {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestMarkWorkloadIdentityFailed(t *testing.T) {
	status := &IdentityStatus{}
	condSet := apis.NewLivingConditionSet()
	status.MarkWorkloadIdentityFailed(&condSet, "failed", "failed")
	got := status.IsReady()
	want := false
	if got != want {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}

func TestMarkWorkloadIdentityUnknown(t *testing.T) {
	status := &IdentityStatus{}
	condSet := apis.NewLivingConditionSet()
	status.MarkWorkloadIdentityUnknown(&condSet, "unknown", "unknown")
	got := status.IsReady()
	want := false
	if got != want {
		t.Errorf("unexpected readiness: want %v, got %v", want, got)
	}
}
