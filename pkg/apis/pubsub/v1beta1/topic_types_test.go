/*
Copyright 2019 The Knative Authors

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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
)

func TestTopicGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "pubsub.cloud.google.com",
		Version: "v1beta1",
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
			IdentitySpec: v1beta1.IdentitySpec{
				GoogleServiceAccount: "test@test",
			},
		},
	}
	want := "test@test"
	got := s.IdentitySpec().GoogleServiceAccount
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopicIdentityStatus(t *testing.T) {
	s := &Topic{
		Status: TopicStatus{
			IdentityStatus: v1beta1.IdentityStatus{},
		},
	}
	want := &v1beta1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestTopicConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: TopicConditionAddressable,
	}, {
		Type: TopicConditionTopicExists,
	}, {
		Type: TopicConditionPublisherReady,
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
