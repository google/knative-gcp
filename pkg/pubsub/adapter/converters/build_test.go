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

	"cloud.google.com/go/pubsub"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	cepubsub "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub/context"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

const (
	buildID     = "c9k3e360-0b36-4df9-b909-3d7810e37a49"
	buildStatus = "SUCCESS"
)

func TestConvertCloudBuild(t *testing.T) {

	tests := []struct {
		name        string
		message     *cepubsub.Message
		sendMode    ModeType
		wantEventFn func() *cloudevents.Event
		wantErr     bool
	}{{
		name: "valid attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"buildId":    buildID,
				"status":     buildStatus,
				"attribute1": "value1",
				"attribute2": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return buildCloudEvent(map[string]string{
				"buildId":    buildID,
				"status":     buildStatus,
				"attribute1": "value1",
				"attribute2": "value2",
			}, buildID, buildStatus)
		},
	},
		{
			name: "no buildId attributes",
			message: &cepubsub.Message{
				Data: []byte("test data"),
				Attributes: map[string]string{
					"status": buildStatus,
				},
			},
			sendMode: Binary,
			wantErr:  true,
		},
		{
			name: "no buildStatus attributes",
			message: &cepubsub.Message{
				Data: []byte("test data"),
				Attributes: map[string]string{
					"buildId": buildID,
				},
			},
			sendMode: Binary,
			wantErr:  true,
		},
		{
			name: "no attributes",
			message: &cepubsub.Message{
				Data:       []byte("test data"),
				Attributes: map[string]string{},
			},
			sendMode: Binary,
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := pubsubcontext.WithTransportContext(context.TODO(), pubsubcontext.NewTransportContext(
				"testproject",
				"testtopic",
				"testsubscription",
				"testmethod",
				&pubsub.Message{
					ID: "id",
				},
			))

			gotEvent, err := Convert(ctx, test.message, test.sendMode, CloudBuildConverter)
			if err != nil {
				if !test.wantErr {
					t.Errorf("converters.convertBuild got error %v want error=%v", err, test.wantErr)
				}
			} else {
				if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
					t.Errorf("converters.convertBuild got unexpeceted cloudevents.Event (-want +got) %s", diff)
				}
			}
		})
	}
}

func buildCloudEvent(extensions map[string]string, buildID, buildStatus string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetSource(v1alpha1.CloudBuildSourceEventSource("testproject", buildID))
	e.SetSubject(buildStatus)
	e.SetDataContentType(cloudevents.ApplicationJSON)
	e.SetType(v1alpha1.CloudBuildSourceEvent)
	e.SetExtension("knativecemode", string(Binary))
	e.SetDataSchema("https://raw.githubusercontent.com/google/knative-gcp/master/schemas/build/schema.json")
	e.Data = []byte("test data")
	e.DataEncoded = true
	for k, v := range extensions {
		e.SetExtension(k, v)
	}
	return &e
}
