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
	"bytes"
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
	buildID     = "c9k3e360-0b36-4df9-b909-3d7810e37a49"
	buildStatus = "SUCCESS"
)

var (
	buildPublishTime = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	data             = []byte("test data")
)

func TestConvertCloudBuild(t *testing.T) {

	tests := []struct {
		name    string
		message *pubsub.Message
		wantErr bool
	}{{
		name: "valid event",
		message: &pubsub.Message{
			ID:          "id",
			PublishTime: buildPublishTime,
			Data:        data,
			Attributes: map[string]string{
				"buildId":    buildID,
				"status":     buildStatus,
				"attribute1": "value1",
				"attribute2": "value2",
			},
		},
	},
		{
			name: "no buildId attributes",
			message: &pubsub.Message{
				Data: data,
				Attributes: map[string]string{
					"status": buildStatus,
				},
			},
			wantErr: true,
		},
		{
			name: "no buildStatus attributes",
			message: &pubsub.Message{
				Data: data,
				Attributes: map[string]string{
					"buildId": buildID,
				},
			},
			wantErr: true,
		},
		{
			name: "no attributes",
			message: &pubsub.Message{
				Data:       data,
				Attributes: map[string]string{},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithProjectKey(context.Background(), "testproject")
			gotEvent, err := NewPubSubConverter().Convert(ctx, test.message, CloudBuild)
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
				if want := schemasv1.CloudBuildSourceEventSource("testproject", buildID); gotEvent.Source() != want {
					t.Errorf("Source %q != %q", gotEvent.Source(), want)
				}
				if gotEvent.Type() != schemasv1.CloudBuildSourceEventType {
					t.Errorf(`Type %q != %q`, gotEvent.Type(), schemasv1.CloudBuildSourceEventType)
				}
				if gotEvent.Subject() != buildStatus {
					t.Errorf("Subject %q != %q", gotEvent.Subject(), buildStatus)
				}
				if gotEvent.DataSchema() != buildSchemaUrl {
					t.Errorf("DataSchema %q != %q", gotEvent.DataSchema(), buildSchemaUrl)
				}
				if !bytes.Equal(gotEvent.Data(), data) {
					t.Errorf("Data %q != %q", gotEvent.Data(), data)
				}
			}
		})
	}
}
