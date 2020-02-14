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
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

func TestConvertCloudPubSub(t *testing.T) {

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
			return pubSubPullCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			}, "")
		},
	}, {
		name: "upper case attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"AttriBUte1": "value1",
				"AttrIbuTe2": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return pubSubPullCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			}, "")
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
			return pubSubPullCloudEvent(map[string]string{
				"attribute1": "value1",
			}, "")
		},
	}, {
		name: "schema as attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
				"schema":     "schema_val",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return pubSubPullCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			}, "schema_val")
		},
	}, {
		name: "Push mode with non valid alphanumeric attribute",
		message: &cepubsub.Message{
			Data: []byte("\"test data\""), // Data passed in quotes for it to be marshalled properly
			Attributes: map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		sendMode: Push,
		wantEventFn: func() *cloudevents.Event {
			return pubSubPushCloudEvent(map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			}, "\"InRlc3QgZGF0YSI=\"")
		},
	}, {
		name: "Push mode with no attributes",
		message: &cepubsub.Message{
			Data:       []byte("\"test data\""), // Data passed in quotes for it to be marshalled properly
			Attributes: map[string]string{},
		},
		sendMode: Push,
		wantEventFn: func() *cloudevents.Event {
			return pubSubPushCloudEvent(nil, "\"InRlc3QgZGF0YSI=\"")
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

			gotEvent, err := Convert(ctx, test.message, test.sendMode, "")
			if err != nil {
				if !test.wantErr {
					t.Errorf("converters.convertPubsub got error %v want error=%v", err, test.wantErr)
				}
			} else {
				if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
					t.Errorf("converters.convertPubsub got unexpeceted cloudevents.Event (-want +got) %s", diff)
				}
			}
		})
	}
}

func pubSubPullCloudEvent(extensions map[string]string, schema string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetSource(v1alpha1.CloudPubSubSourceEventSource("testproject", "testtopic"))
	e.SetDataContentType("application/octet-stream")
	e.SetType(v1alpha1.CloudPubSubSourcePublish)
	e.SetExtension("knativecemode", string(Binary))
	e.Data = []byte("test data")
	e.DataEncoded = true
	for k, v := range extensions {
		e.SetExtension(k, v)
	}
	if schema != "" {
		e.SetDataSchema(schema)
	}
	return &e
}

func pubSubPushCloudEvent(attributes map[string]string, data string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetSource(v1alpha1.CloudPubSubSourceEventSource("testproject", "testtopic"))
	e.SetDataContentType(cloudevents.ApplicationJSON)
	e.SetType(v1alpha1.CloudPubSubSourcePublish)
	e.SetExtension("knativecemode", string(Push))
	at := ""
	if attributes != nil {
		ex, _ := json.Marshal(attributes)
		at = fmt.Sprintf(`"attributes":%s,`, ex)
	}
	s := fmt.Sprintf(`{"subscription":"testsubscription","message":{"messageId":"id","data":%s,%s"publishTime":"0001-01-01T00:00:00Z"}}`, data, at)
	e.Data = []byte(s)
	e.DataEncoded = true
	return &e
}
