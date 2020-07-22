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

	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/apis"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
)

func TestChannelGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "messaging.cloud.google.com",
		Version: "v1beta1",
		Kind:    "Channel",
	}

	c := &Channel{}
	got := c.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestChannelIdentitySpec(t *testing.T) {
	s := &Channel{
		Spec: ChannelSpec{
			IdentitySpec: duckv1.IdentitySpec{
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

func TestChannelIdentityStatus(t *testing.T) {
	s := &Channel{
		Status: ChannelStatus{
			IdentityStatus: duckv1.IdentityStatus{},
		},
	}
	want := &duckv1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestChannelConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: ChannelConditionAddressable,
	}, {
		Type: ChannelConditionTopicReady,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &Channel{}

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

func TestChannel_GetConditionSet(t *testing.T) {
	c := &Channel{}

	if got, want := c.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestChannel_GetStatus(t *testing.T) {
	c := &Channel{
		Status: ChannelStatus{},
	}
	if got, want := c.GetStatus(), &c.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}
