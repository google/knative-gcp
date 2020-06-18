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

package converters

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"

	cev2 "github.com/cloudevents/sdk-go/v2"
)

func TestConvertCloudSchedulerSource(t *testing.T) {

	tests := []struct {
		name        string
		message     *pubsub.Message
		source      string
		wantEventFn func() *cev2.Event
		wantErr     string
	}{{
		name: "valid attributes",
		message: &pubsub.Message{
			ID:   "id",
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":   "com.google.cloud.scheduler",
				"jobName":       "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"schedulerName": "scheduler-test",
				"attribute1":    "value1",
				"attribute2":    "value2",
			},
		},
		wantEventFn: func() *cev2.Event {
			return schedulerCloudEvent(
				"//cloudscheduler.googleapis.com/projects/knative-gcp-test/locations/us-east4/schedulers/scheduler-test",
				"jobs/cre-scheduler-test")
		},
	}, {
		name: "missing jobName attribute",
		message: &pubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":   "com.google.cloud.scheduler",
				"schedulerName": "scheduler-test",
				"attribute1":    "value1",
				"attribute2":    "value2",
			},
		},
		wantErr: "received event did not have jobName",
	}, {
		name: "missing schedulerName attribute",
		message: &pubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.scheduler",
				"jobName":     "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"attribute1":  "value1",
				"attribute2":  "value2",
			},
		},
		wantErr: "received event did not have schedulerName",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			gotEvent, err := NewPubSubConverter().Convert(context.Background(), test.message, CloudScheduler)

			if test.wantErr != "" || err != nil {
				var gotErr string
				if err != nil {
					gotErr = err.Error()
				}
				if !strings.Contains(test.wantErr, gotErr) {
					diff := cmp.Diff(test.wantErr, gotErr)
					t.Errorf("unexpected error (-want, +got) = %v", diff)
				}
				return
			}

			if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
				t.Errorf("converters.convertCloudScheduler got unexpected cev2.Event (-want +got) %s", diff)
			}
		})
	}
}

func schedulerCloudEvent(source, subject string) *cev2.Event {
	e := cev2.NewEvent(cev2.VersionV1)
	e.SetID("id")
	e.SetData(cev2.ApplicationJSON, []byte("test data"))
	e.SetType(v1beta1.CloudSchedulerSourceExecute)
	e.SetSource(source)
	e.SetSubject(subject)
	return &e
}
