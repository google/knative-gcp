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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

func TestCloudSchedulerSourceGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "CloudSchedulerSource",
	}

	s := &CloudSchedulerSource{}
	got := s.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSourceEventSource(t *testing.T) {
	want := "//cloudscheduler.googleapis.com/PARENT/schedulers/SCHEDULER"

	got := CloudSchedulerSourceEventSource("PARENT", "SCHEDULER")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSourceConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: JobReady,
	}, {
		Type: v1alpha1.TopicReady,
	}, {
		Type: v1alpha1.PullSubscriptionReady,
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
			PubSubSpec: v1alpha1.PubSubSpec{
				IdentitySpec: v1alpha1.IdentitySpec{
					GoogleServiceAccount: "test@test",
				},
			},
		},
	}
	want := "test@test"
	got := s.IdentitySpec().GoogleServiceAccount
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudSchedulerSourceIdentityStatus(t *testing.T) {
	s := &CloudSchedulerSource{
		Status: CloudSchedulerSourceStatus{
			PubSubStatus: v1alpha1.PubSubStatus{},
		},
	}
	want := &v1alpha1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
