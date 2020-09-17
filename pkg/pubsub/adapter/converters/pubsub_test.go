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
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

func TestConvertCloudPubSub(t *testing.T) {

	tests := []struct {
		name               string
		message            *pubsub.Message
		wantEventFn        func() *cev2.Event
		wantErr            bool
		wantInvalidContext bool
	}{{
		name: "non alphanumeric attribute",
		message: &pubsub.Message{
			ID:   "id",
			Data: []byte("\"test data\""), // Data passed in quotes for it to be marshalled properly
			Attributes: map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		wantEventFn: func() *cev2.Event {
			return pubSubCloudEvent(map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			}, "\"InRlc3QgZGF0YSI=\"")
		},
	}, {
		name: "no attributes",
		message: &pubsub.Message{
			ID:         "id",
			Data:       []byte("\"test data\""), // Data passed in quotes for it to be marshalled properly
			Attributes: map[string]string{},
		},
		wantEventFn: func() *cev2.Event {
			return pubSubCloudEvent(nil, "\"InRlc3QgZGF0YSI=\"")
		},
	}, {
		name: "invalid context",
		message: &pubsub.Message{
			ID:         "id",
			Data:       []byte("\"test data\""), // Data passed in quotes for it to be marshalled properly
			Attributes: map[string]string{},
		},
		wantInvalidContext: true,
		wantErr:            true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			// TODO add flags to test each of the keys
			if !test.wantInvalidContext {
				ctx = WithProjectKey(context.Background(), "testproject")
				ctx = WithTopicKey(ctx, "testtopic")
				ctx = WithSubscriptionKey(ctx, "testsubscription")
			}

			gotEvent, err := NewPubSubConverter().Convert(ctx, test.message, CloudPubSub)
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

func pubSubCloudEvent(attributes map[string]string, data string) *cev2.Event {
	e := cev2.NewEvent(cev2.VersionV1)
	e.SetID("id")
	e.SetSource(schemasv1.CloudPubSubEventSource("testproject", "testtopic"))
	at := ""
	if attributes != nil {
		ex, _ := json.Marshal(attributes)
		at = fmt.Sprintf(`"attributes":%s,`, ex)
	}
	s := fmt.Sprintf(`{"subscription":"testsubscription","message":{"messageId":"id","data":%s,%s"publishTime":"0001-01-01T00:00:00Z"}}`, data, at)
	e.SetData(cev2.ApplicationJSON, []byte(s))
	e.SetType(schemasv1.CloudPubSubMessagePublishedEventType)
	e.SetDataSchema(schemasv1.CloudPubSubEventDataSchema)
	e.DataBase64 = false
	return &e
}
