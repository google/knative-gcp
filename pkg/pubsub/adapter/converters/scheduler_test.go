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
	cloudevents "github.com/cloudevents/sdk-go/legacy"
	cepubsub "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/pubsub/context"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

func TestConvertCloudSchedulerSource(t *testing.T) {

	tests := []struct {
		name        string
		message     *cepubsub.Message
		source      string
		sendMode    ModeType
		wantEventFn func() *cloudevents.Event
		wantErr     string
	}{{
		name: "valid attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":   "com.google.cloud.scheduler",
				"jobName":       "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"schedulerName": "scheduler-test",
				"attribute1":    "value1",
				"attribute2":    "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return schedulerCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2"},
				"//cloudscheduler.googleapis.com/projects/knative-gcp-test/locations/us-east4/schedulers/scheduler-test",
				"jobs/cre-scheduler-test")
		},
	}, {
		name: "upper case attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":   "com.google.cloud.scheduler",
				"jobName":       "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"schedulerName": "scheduler-test",
				"AttriBUte1":    "value1",
				"AttrIbuTe2":    "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return schedulerCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			},
				"//cloudscheduler.googleapis.com/projects/knative-gcp-test/locations/us-east4/schedulers/scheduler-test",
				"jobs/cre-scheduler-test")
		},
	}, {
		name: "only setting valid alphanumeric attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":       "com.google.cloud.scheduler",
				"jobName":           "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"schedulerName":     "scheduler-test",
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return schedulerCloudEvent(map[string]string{
				"attribute1": "value1"},
				"//cloudscheduler.googleapis.com/projects/knative-gcp-test/locations/us-east4/schedulers/scheduler-test",
				"jobs/cre-scheduler-test")
		},
	}, {
		name: "missing jobName attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":   "com.google.cloud.scheduler",
				"schedulerName": "scheduler-test",
				"attribute1":    "value1",
				"attribute2":    "value2",
			},
		},
		sendMode: Binary,
		wantErr:  "received event did not have jobName",
	}, {
		name: "missing schedulerName attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.scheduler",
				"jobName":     "projects/knative-gcp-test/locations/us-east4/jobs/cre-scheduler-test",
				"attribute1":  "value1",
				"attribute2":  "value2",
			},
		},
		sendMode: Binary,
		wantErr:  "received event did not have schedulerName",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := pubsubcontext.WithTransportContext(context.Background(), pubsubcontext.NewTransportContext(
				"testproject",
				"testtopic",
				"testsubscription",
				"testmethod",
				&pubsub.Message{
					ID: "id",
				},
			))

			gotEvent, err := Convert(ctx, test.message, test.sendMode, "")

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
				t.Errorf("converters.convertCloudScheduler got unexpeceted cloudevents.Event (-want +got) %s", diff)
			}
		})
	}
}

func schedulerCloudEvent(extensions map[string]string, source, subject string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetDataContentType("application/octet-stream")
	e.SetType(v1alpha1.CloudSchedulerSourceExecute)
	e.SetExtension("knativecemode", string(Binary))
	e.SetSource(source)
	e.SetSubject(subject)
	e.Data = []byte("test data")
	e.DataEncoded = true
	for k, v := range extensions {
		if k != v1alpha1.CloudSchedulerSourceJobName {
			e.SetExtension(k, v)
		}
	}
	return &e
}
