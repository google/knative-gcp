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

package v1

import (
	"testing"
)

func TestCloudAuditLogsEventSource(t *testing.T) {
	want := "//cloudaudit.googleapis.com/projects/PROJECT/logs/activity"
	got := CloudAuditLogsEventSource("projects/PROJECT", "activity")
	if got != want {
		t.Errorf("CloudAuditLogsEventSource got=%s, want=%s", got, want)
	}
}

func TestCloudAuditLogsEventSubject(t *testing.T) {
	want := "pubsub.googleapis.com/projects/PROJECT/topics/TOPIC"
	got := CloudAuditLogsEventSubject("pubsub.googleapis.com", "projects/PROJECT/topics/TOPIC")
	if got != want {
		t.Errorf("CloudAuditLogsEventSubject got=%s, want=%s", got, want)
	}
}

func TestCloudAuditLogsventID(t *testing.T) {
	want := "efdb9bf7d6fdfc922352530c1ba51242"
	got := CloudAuditLogsEventID("pt9y76cxw5", "projects/knative-project-228222/logs/cloudaudit.googleapis.com%2Factivity", "2020-01-19T22:45:03.439395442Z")
	if got != want {
		t.Errorf("CloudAuditLogsEventID got=%s, want=%s", got, want)
	}
}
