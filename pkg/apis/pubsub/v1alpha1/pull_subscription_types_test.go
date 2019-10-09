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
	"time"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPubSubEventSource(t *testing.T) {
	want := "//pubsub.googleapis.com/projects/PROJECT/topics/TOPIC"

	got := PubSubEventSource("PROJECT", "TOPIC")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestPullSubscriptionGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "pubsub.cloud.google.com",
		Version: "v1alpha1",
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
