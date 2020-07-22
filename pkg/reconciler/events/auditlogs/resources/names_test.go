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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateTopicName(t *testing.T) {
	want := "cre-src_mynamespace_myname_uid"
	got := GenerateTopicName(&v1.CloudAuditLogsSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "mynamespace",
			UID:       "uid",
		},
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGenerateTopicResourceName(t *testing.T) {
	want := "pubsub.googleapis.com/projects/project/topics/topic"
	got := GenerateTopicResourceName(&v1.CloudAuditLogsSource{
		Status: v1.CloudAuditLogsSourceStatus{
			PubSubStatus: duckv1.PubSubStatus{
				ProjectID: "project",
				TopicID:   "topic",
			},
		},
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGenerateSinkName(t *testing.T) {
	want := "cre-src_mynamespace_myname_uid"
	got := GenerateSinkName(&v1.CloudAuditLogsSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "mynamespace",
			UID:       "uid",
		},
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
