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
	want := "cloud-run-events-channel=controller,cloud-run-events-channel-name=source"
	got := GetLabelSelector("controller", "source").String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetLabels(t *testing.T) {
	want := map[string]string{
		"cloud-run-events-channel":      "controller",
		"cloud-run-events-channel-name": "channel",
	}
	got := GetLabels("controller", "channel")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetPullSubscriptionLabelSelector(t *testing.T) {
	want := "cloud-run-events-channel=controller,cloud-run-events-channel-name=source,cloud-run-events-channel-subscriber=subscriber"
	got := GetPullSubscriptionLabelSelector("controller", "source", "subscriber").String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGetPullSubscriptionLabels(t *testing.T) {
	want := map[string]string{
		"cloud-run-events-channel":            "controller",
		"cloud-run-events-channel-name":       "channel",
		"cloud-run-events-channel-subscriber": "subscriber",
	}
	got := GetPullSubscriptionLabels("controller", "channel", "subscriber")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
