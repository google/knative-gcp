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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

func TestAuditLogsGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1alpha1",
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
		Type: duckv1alpha1.TopicReady,
	}, {
		Type: duckv1alpha1.PullSubscriptionReady,
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

func TestCloudAuditLogsSourceEventSource(t *testing.T) {
	want := "//pubsub.googleapis.com/projects/PROJECT"

	got := CloudAuditLogsSourceEventSource("pubsub.googleapis.com", "projects/PROJECT")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudAuditLogsSourceEventID(t *testing.T) {
	want := "efdb9bf7d6fdfc922352530c1ba51242"

	got := CloudAuditLogsSourceEventID("pt9y76cxw5", "projects/knative-project-228222/logs/cloudaudit.googleapis.com%2Factivity", "2020-01-19T22:45:03.439395442Z")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudAuditLogsSourceIdentitySpec(t *testing.T) {
	s := &CloudAuditLogsSource{
		Spec: CloudAuditLogsSourceSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
				IdentitySpec: duckv1alpha1.IdentitySpec{
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

func TestCloudAuditLogsSourceIdentityStatus(t *testing.T) {
	s := &CloudAuditLogsSource{
		Status: CloudAuditLogsSourceStatus{
			PubSubStatus: duckv1alpha1.PubSubStatus{},
		},
	}
	want := &duckv1alpha1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
