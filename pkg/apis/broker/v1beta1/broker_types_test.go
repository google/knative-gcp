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
	"knative.dev/pkg/apis"
)

func TestBroker_GetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1beta1",
		Kind:    "Broker",
	}
	b := Broker{}
	got := b.GetGroupVersionKind()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(GetGroupVersionKind (-want +got): %v", diff)
	}
}

func TestBroker_GetUntypedSpec(t *testing.T) {
	b := Broker{
		Spec: eventingv1beta1.BrokerSpec{},
	}
	s := b.GetUntypedSpec()
	if _, ok := s.(eventingv1beta1.BrokerSpec); !ok {
		t.Errorf("untyped spec was not a BrokerSpec")
	}
}

func TestBroker_GetConditionSet(t *testing.T) {
	b := &Broker{}

	if got, want := b.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestBroker_GetStatus(t *testing.T) {
	b := &Broker{
		Status: BrokerStatus{},
	}
	if got, want := b.GetStatus(), &b.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}
