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
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"
)

func TestEventPolicyBindingGetGroupVersionKind(t *testing.T) {
	pb := EventPolicyBinding{}
	gvk := pb.GetGroupVersionKind()
	if gvk.Kind != "EventPolicyBinding" {
		t.Errorf("EventPolicy.GetGroupVersionKind.Kind want=EventPolicyBinding got=%s", gvk.Kind)
	}
}

func TestEventPolicyBindingGetSubject(t *testing.T) {
	sub := tracker.Reference{
		APIVersion: "foo.bar",
		Kind:       "PolicyBinding",
		Name:       "foo",
		Namespace:  "bar",
	}
	pb := &EventPolicyBinding{
		Spec: PolicyBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: sub,
			},
		},
	}
	gotSub := pb.GetSubject()
	if diff := cmp.Diff(sub, gotSub); diff != "" {
		t.Errorf("EventPolicyBinding.GetSubject() (-want, +got) = %v", diff)
	}
}

func TestEventPolicyBindingGetBindingStatus(t *testing.T) {
	s := &PolicyBindingStatus{}
	s.InitializeConditions()
	s.MarkBindingAvailable()

	pb := &EventPolicyBinding{
		Status: *s,
	}

	gotStatus := pb.GetBindingStatus()
	if diff := cmp.Diff(s, gotStatus); diff != "" {
		t.Errorf("EventPolicyBinding.GetBindingStatus() (-want, +got) = %v", diff)
	}
}
