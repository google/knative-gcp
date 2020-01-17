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

func TestPubSubGetGroupVersionKind(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "events.cloud.google.com",
		Version: "v1alpha1",
		Kind:    "PubSub",
	}

	c := &PubSub{}
	got := c.GetGroupVersionKind()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline(t *testing.T) {
	want := 10 * time.Second
	s := &PubSubSpec{AckDeadline: ptr.String("10s")}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration(t *testing.T) {
	want := 10 * time.Second
	s := &PubSubSpec{RetentionDuration: ptr.String("10s")}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetAckDeadline_default(t *testing.T) {
	want := defaultAckDeadline
	s := &PubSubSpec{}
	got := s.GetAckDeadline()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestGetRetentionDuration_default(t *testing.T) {
	want := defaultRetentionDuration
	s := &PubSubSpec{}
	got := s.GetRetentionDuration()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
