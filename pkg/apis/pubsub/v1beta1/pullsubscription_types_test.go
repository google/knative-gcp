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
	"time"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPullSubscriptionGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "pubsub.cloud.google.com",
		Version: "v1beta1",
		Kind:    "PullSubscription",
	}

	c := &PullSubscription{}
	got := c.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestPullSubscriptionPubSubMode_nil(t *testing.T) {
	want := ModeType("")

	c := &PullSubscription{}
	got := c.PubSubMode()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline(t *testing.T) {
	want := 10 * time.Second
	s := &PullSubscriptionSpec{AckDeadline: ptr.String("10s")}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration(t *testing.T) {
	want := 10 * time.Second
	s := &PullSubscriptionSpec{RetentionDuration: ptr.String("10s")}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline_default(t *testing.T) {
	want := defaultAckDeadline
	s := &PullSubscriptionSpec{}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration_default(t *testing.T) {
	want := defaultRetentionDuration
	s := &PullSubscriptionSpec{}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestPullSubscriptionIdentitySpec(t *testing.T) {
	s := &PullSubscription{
		Spec: PullSubscriptionSpec{
			PubSubSpec: v1beta1.PubSubSpec{
				IdentitySpec: v1beta1.IdentitySpec{
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

func TestPullSubscriptionIdentityStatus(t *testing.T) {
	s := &PullSubscription{
		Status: PullSubscriptionStatus{
			PubSubStatus: v1beta1.PubSubStatus{
				IdentityStatus: v1beta1.IdentityStatus{},
			},
		},
	}
	want := &v1beta1.IdentityStatus{}
	got := s.IdentityStatus()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestPullSubscriptionConditionSet(t *testing.T) {
	want := []apis.Condition{{
		Type: PullSubscriptionConditionSinkProvided,
	}, {
		Type: PullSubscriptionConditionDeployed,
	}, {
		Type: PullSubscriptionConditionSubscribed,
	}, {
		Type: apis.ConditionReady,
	}}
	c := &PullSubscription{}

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
