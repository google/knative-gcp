/*
Copyright 2020 Google LLC

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

package metrics

import (
	"testing"
)

func TestEventTypeMetricValue(t *testing.T) {
	tests := []struct {
		event string
		want  string
	}{
		// Current Supported GCP event types
		{"google.cloud.audit.log.v1.written", "google.cloud.audit.log.v1.written"},
		{"google.cloud.pubsub.topic.v1.messagePublished", "google.cloud.pubsub.topic.v1.messagePublished"},
		{"google.cloud.scheduler.job.v1.executed", "google.cloud.scheduler.job.v1.executed"},
		{"google.cloud.storage.object.v1.archived", "google.cloud.storage.object.v1.archived"},
		{"google.cloud.storage.object.v1.deleted", "google.cloud.storage.object.v1.deleted"},
		{"google.cloud.storage.object.v1.finalized", "google.cloud.storage.object.v1.finalized"},
		{"google.cloud.storage.object.v1.metadataUpdated", "google.cloud.storage.object.v1.metadataUpdated"},

		{"google.cloud.some.newly.added.event", "google.cloud.some.newly.added.event"},

		{"some.custom.event", "custom"},
		{"", "custom"},

		// Used to mark invalid cloud events
		{"_invalid_cloud_event_", "_invalid_cloud_event_"},

		// Used in E2E tests
		{"e2e-sample-event-type", "e2e-sample-event-type"},
		{"e2e-testing-resp-event-type-sample", "e2e-testing-resp-event-type-sample"},
	}

	for _, test := range tests {
		if got := EventTypeMetricValue(test.event); got != test.want {
			t.Errorf("EventTypeMetricValue(%q) = %v; want %v", test.event, got, test.want)
		}
	}
}
