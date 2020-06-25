/*
Copyright 2019 Google LLC.

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
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

func TestAuditLogsGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1beta1",
		Kind:    "CloudAuditLogsSource",
	}

	c := &CloudAuditLogsSource{}

	got := c.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestAuditLogsConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: SinkReady,
	}, {
		Type: duckv1beta1.TopicReady,
	}, {
		Type: duckv1beta1.PullSubscriptionReady,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &CloudAuditLogsSource{}

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

func TestCloudAuditLogsSourceIdentitySpec(t *testing.T) {
	s := &CloudAuditLogsSource{
		Spec: CloudAuditLogsSourceSpec{
			PubSubSpec: duckv1beta1.PubSubSpec{
				IdentitySpec: duckv1beta1.IdentitySpec{
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

func TestCloudAuditLogsSourceIdentityStatus(t *testing.T) {
	s := &CloudAuditLogsSource{
		Status: CloudAuditLogsSourceStatus{
			PubSubStatus: duckv1beta1.PubSubStatus{},
		},
	}
	want := &duckv1beta1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudAuditLogsSource_GetConditionSet(t *testing.T) {
	s := &CloudAuditLogsSource{}

	if got, want := s.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestCloudAuditLogsSource_GetStatus(t *testing.T) {
	s := &CloudAuditLogsSource{
		Status: CloudAuditLogsSourceStatus{},
	}
	if got, want := s.GetStatus(), &s.Status.Status; got != want {
		t.Errorf("GetStatus=%v, want=%v", got, want)
	}
}
