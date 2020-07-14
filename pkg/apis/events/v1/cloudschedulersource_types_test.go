/*
Copyright 2020 The Google LLC

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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

func TestCloudSchedulerSourceGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1",
		Kind:    "CloudSchedulerSource",
	}

	s := &CloudSchedulerSource{}
	got := s.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSourceConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: JobReady,
	}, {
		Type: duckv1.TopicReady,
	}, {
		Type: duckv1.PullSubscriptionReady,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &CloudSchedulerSource{}

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

func TestCloudSchedulerSourceIdentitySpec(t *testing.T) {
	s := &CloudSchedulerSource{
		Spec: CloudSchedulerSourceSpec{
			PubSubSpec: duckv1.PubSubSpec{
				IdentitySpec: duckv1.IdentitySpec{
					ServiceAccountName: "test",
				},
			},
		},
	}
	want := "test"
	got := s.IdentitySpec().ServiceAccountName
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSourceIdentityStatus(t *testing.T) {
	s := &CloudSchedulerSource{
		Status: CloudSchedulerSourceStatus{
			PubSubStatus: duckv1.PubSubStatus{},
		},
	}
	want := &duckv1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSource_GetConditionSet(t *testing.T) {
	s := &CloudSchedulerSource{}

	if got, want := s.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestCloudSchedulerSource_GetStatus(t *testing.T) {
	s := &CloudSchedulerSource{
		Status: CloudSchedulerSourceStatus{},
	}
	if got, want := s.GetStatus(), &s.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}
