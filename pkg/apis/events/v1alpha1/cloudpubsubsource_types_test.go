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
	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/apis"
	"testing"
	"time"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCloudPubSubSourceEventSource(t *testing.T) {
	want := "//pubsub.googleapis.com/projects/PROJECT/topics/TOPIC"

	got := CloudPubSubSourceEventSource("PROJECT", "TOPIC")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudPubSubSourceGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "CloudPubSubSource",
	}

	c := &CloudPubSubSource{}
	got := c.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline(t *testing.T) {
	want := 10 * time.Second
	s := &CloudPubSubSourceSpec{AckDeadline: ptr.String("10s")}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration(t *testing.T) {
	want := 10 * time.Second
	s := &CloudPubSubSourceSpec{RetentionDuration: ptr.String("10s")}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline_default(t *testing.T) {
	want := defaultAckDeadline
	s := &CloudPubSubSourceSpec{}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration_default(t *testing.T) {
	want := defaultRetentionDuration
	s := &CloudPubSubSourceSpec{}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudPubSubSourceIdentitySpec(t *testing.T) {
	s := &CloudPubSubSource{
		Spec: CloudPubSubSourceSpec{
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

func TestCloudPubSubSourceIdentityStatus(t *testing.T) {
	s := &CloudPubSubSource{
		Status: CloudPubSubSourceStatus{
			PubSubStatus: v1alpha1.PubSubStatus{},
		},
	}
	want := &v1alpha1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestCloudPubSubSourceConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: v1alpha1.PullSubscriptionReady,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &CloudPubSubSource{}

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
