/*
Copyright 2019 Google LLC

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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetLabelSelector(t *testing.T) {
	want := "events.cloud.google.com/channel=controller,events.cloud.google.com/channel-name=source,events.cloud.google.com/controller-uid=UID"
	got := GetLabelSelector("controller", "source", "UID").String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetLabels(t *testing.T) {
	want := map[string]string{
		"events.cloud.google.com/channel":        "controller",
		"events.cloud.google.com/channel-name":   "channel",
		"events.cloud.google.com/controller-uid": "UID",
	}
	got := GetLabels("controller", "channel", "UID")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetPullSubscriptionLabelSelector(t *testing.T) {
	want := "events.cloud.google.com/channel=controller,events.cloud.google.com/channel-name=source,events.cloud.google.com/channel-subscriber=subscriber,events.cloud.google.com/controller-uid=UID"
	got := GetPullSubscriptionLabelSelector("controller", "source", "subscriber", "UID").String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetPullSubscriptionLabels(t *testing.T) {
	want := map[string]string{
		"events.cloud.google.com/channel":            "controller",
		"events.cloud.google.com/channel-name":       "channel",
		"events.cloud.google.com/channel-subscriber": "subscriber",
		"events.cloud.google.com/controller-uid":     "UID",
	}
	got := GetPullSubscriptionLabels("controller", "channel", "subscriber", "UID")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
