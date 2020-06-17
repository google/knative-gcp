/*
Copyright 2020 Google LLC.

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
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
)

const (
	buildID     = "c9k3e360-0b36-4df9-b909-3d7810e37a49"
	buildStatus = "SUCCESS"
)

var (
	buildPublishTime = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
)

func TestConvertCloudBuild(t *testing.T) {

	tests := []struct {
		name        string
		message     *pubsub.Message
		wantEventFn func() *cev2.Event
		wantErr     bool
	}{{
		name: "valid event",
		message: &pubsub.Message{
			ID:          "id",
			PublishTime: buildPublishTime,
			Data:        []byte("test data"),
			Attributes: map[string]string{
				"buildId":    buildID,
				"status":     buildStatus,
				"attribute1": "value1",
				"attribute2": "value2",
			},
		},
		wantEventFn: func() *cev2.Event {
			return buildCloudEvent(map[string]string{
				"buildId": buildID,
				"status":  buildStatus,
			}, buildID, buildStatus)
		},
	},
		{
			name: "no buildId attributes",
			message: &pubsub.Message{
				Data: []byte("test data"),
				Attributes: map[string]string{
					"status": buildStatus,
				},
			},
			wantErr: true,
		},
		{
			name: "no buildStatus attributes",
			message: &pubsub.Message{
				Data: []byte("test data"),
				Attributes: map[string]string{
					"buildId": buildID,
				},
			},
			wantErr: true,
		},
		{
			name: "no attributes",
			message: &pubsub.Message{
				Data:       []byte("test data"),
				Attributes: map[string]string{},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithProjectKey(context.Background(), "testproject")
			gotEvent, err := Convert(ctx, test.message, CloudBuildConverter)
			if err != nil {
				if !test.wantErr {
					t.Errorf("converters.convertBuild got error %v want error=%v", err, test.wantErr)
				}
			} else {
				if gotEvent.ID() != "id" {
					t.Errorf("ID '%s' != '%s'", gotEvent.ID(), "id")
				}
				if !gotEvent.Time().Equal(buildPublishTime) {
					t.Errorf("Time '%v' != '%v'", gotEvent.Time(), buildPublishTime)
				}
				if want := v1beta1.CloudBuildSourceEventSource("testproject", buildID); gotEvent.Source() != want {
					t.Errorf("Source %q != %q", gotEvent.Source(), want)
				}
				if gotEvent.Type() != v1beta1.CloudBuildSourceEvent {
					t.Errorf(`Type %q != %q`, gotEvent.Type(), v1beta1.CloudBuildSourceEvent)
				}
				if gotEvent.Subject() != buildStatus {
					t.Errorf("Subject %q != %q", gotEvent.Subject(), buildStatus)
				}
				if gotEvent.DataSchema() != buildSchemaUrl {
					t.Errorf("DataSchema %q != %q", gotEvent.DataSchema(), buildSchemaUrl)
				}
			}
		})
	}
}

func buildCloudEvent(extensions map[string]string, buildID, buildStatus string) *cev2.Event {
	e := cev2.NewEvent(cev2.VersionV1)
	e.SetID("id")
	e.SetTime(buildPublishTime)
	e.SetSource(v1beta1.CloudBuildSourceEventSource("testproject", buildID))
	e.SetSubject(buildStatus)
	e.SetData(cev2.ApplicationJSON, []byte("test data"))
	e.SetType(v1beta1.CloudBuildSourceEvent)
	e.SetDataSchema(buildSchemaUrl)
	return &e
}
