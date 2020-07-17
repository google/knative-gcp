/*
Copyright 2020 The Google LLC.

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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
)

func TestTopicGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "internal.events.cloud.google.com",
		Version: "v1",
		Kind:    "Topic",
	}

	p := &Topic{}
	got := p.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopicIdentitySpec(t *testing.T) {
	s := &Topic{
		Spec: TopicSpec{
			IdentitySpec: v1.IdentitySpec{
				ServiceAccountName: "test",
			},
		},
	}
	want := "test"
	got := s.IdentitySpec().ServiceAccountName
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopicIdentityStatus(t *testing.T) {
	s := &Topic{
		Status: TopicStatus{
			IdentityStatus: v1.IdentityStatus{},
		},
	}
	want := &v1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopicConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: TopicConditionTopicExists,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &Topic{}

	c.ConditionSet().Manage(&c.Status).InitializeConditions()
	var got []apis.Condition = c.Status.GetConditions()

	compareConditionTypes := cmp.Transformer("ConditionType", func(c apis.Condition) apis.ConditionType {
		return c.Type
	})
	sortConditionTypes := cmpopts.SortSlices(func(a, b apis.Condition) bool {
		return a.Type < b.Type
	})
	if diff := cmp.Diff(want, got, sortConditionTypes, compareConditionTypes); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopic_GetConditionSet(t *testing.T) {
	tp := &Topic{}

	if got, want := tp.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestTopic_GetStatus(t *testing.T) {
	tp := &Topic{
		Status: TopicStatus{},
	}
	if got, want := tp.GetStatus(), &tp.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}
