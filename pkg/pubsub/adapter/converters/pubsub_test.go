/*
Copyright 2019 Google LLC

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
	"cloud.google.com/go/pubsub"
	"context"
	"github.com/google/go-cmp/cmp"
	"testing"

	"github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

func TestConvertPubSub(t *testing.T) {

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
				"attribute1": "value1",
				"attribute2": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return cloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			})
		},
	}, {
		name: "invalid upper case attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"AttriBUte1": "value1",
				"AttrIbuTe2": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return cloudEvent(map[string]string{})
		},
	}, {
		name: "only setting valid alphanumeric attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return cloudEvent(map[string]string{
				"attribute1": "value1",
			})
		},
	}}

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

			gotEvent, err := convertPubsub(ctx, test.message, test.sendMode)

			if (err != nil) != test.wantErr {
				t.Errorf("converters.convertPubsub got error %v want error=%v", err, test.wantErr)
			}
			if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
				t.Errorf("converters.convertPubsub got unexpeceted cloudevents.Event (-want +got) %s", diff)
			}
		})
	}
}

func cloudEvent(extensions map[string]string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetSource(v1alpha1.PubSubEventSource("testproject", "testtopic"))
	e.SetDataContentType("application/octet-stream")
	e.SetType(v1alpha1.PubSubPublish)
	e.SetExtension("knativecemode", string(Binary))
	e.Data = []byte("test data")
	e.DataEncoded = true
	for k, v := range extensions {
		e.SetExtension(k, v)
	}
	return &e
}
