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
		{"com.google.cloud.auditlog.event", "com.google.cloud.auditlog.event"},
		{"com.google.cloud.pubsub.topic.publish", "com.google.cloud.pubsub.topic.publish"},
		{"com.google.cloud.scheduler.job.execute", "com.google.cloud.scheduler.job.execute"},
		{"com.google.cloud.storage.object.archive", "com.google.cloud.storage.object.archive"},
		{"com.google.cloud.storage.object.delete", "com.google.cloud.storage.object.delete"},
		{"com.google.cloud.storage.object.finalize", "com.google.cloud.storage.object.finalize"},
		{"com.google.cloud.storage.object.metadataUpdate", "com.google.cloud.storage.object.metadataUpdate"},

		{"com.google.cloud.some.newly.added.event", "com.google.cloud.some.newly.added.event"},

		{"some.custom.event", "custom"},
		{"", "custom"},

		// Used in E2E tests
		{"e2e-dummy-event-type", "e2e-dummy-event-type"},
		{"e2e-testing-resp-event-type-dummy", "e2e-testing-resp-event-type-dummy"},
	}

	for _, test := range tests {
		if got := EventTypeMetricValue(test.event); got != test.want {
			t.Errorf("EventTypeMetricValue(%q) = %v; want %v", test.event, got, test.want)
		}
	}
}
