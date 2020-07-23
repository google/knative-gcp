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

func TestGenerateJobName(t *testing.T) {
	want := "projects/project/locations/location/" + JobPrefix + "-uid"
	got := GenerateJobName(&v1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		Spec: v1.CloudSchedulerSourceSpec{
			Location: "location",
		},
		Status: v1.CloudSchedulerSourceStatus{
			PubSubStatus: duckv1.PubSubStatus{
				ProjectID: "project",
			},
		},
	})

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestExtractParentName(t *testing.T) {
	want := "projects/PROJECT_ID/locations/LOCATION_ID"
	got := ExtractParentName("projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestExtractJobId(t *testing.T) {
	want := "jobs/JOB_ID"
	got := ExtractJobID("projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestGenerateTopicName(t *testing.T) {
	want := "cre-src_mynamespace_myname_uid"
	got := GenerateTopicName(&v1.CloudSchedulerSource{
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

func TestGeneratePubSubTargetTopic(t *testing.T) {
	want := "projects/project/topics/topic"
	got := GeneratePubSubTargetTopic(&v1.CloudSchedulerSource{
		Status: v1.CloudSchedulerSourceStatus{
			PubSubStatus: duckv1.PubSubStatus{
				ProjectID: "project",
			},
		},
	}, "topic")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
