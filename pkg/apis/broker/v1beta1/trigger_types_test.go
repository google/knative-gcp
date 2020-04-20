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

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

func TestTrigger_GetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1beta1",
		Kind:    "Trigger",
	}
	trig := Trigger{}
	got := trig.GetGroupVersionKind()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(GetGroupVersionKind (-want +got): %v", diff)
	}
}

func TestTrigger_GetUntypedSpec(t *testing.T) {
	b := Trigger{
		Spec: eventingv1beta1.TriggerSpec{},
	}
	s := b.GetUntypedSpec()
	if _, ok := s.(eventingv1beta1.TriggerSpec); !ok {
		t.Errorf("untyped spec was not a TriggerSpec")
	}
}
