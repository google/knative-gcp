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
	"testing"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

func TestConvertPubSubPull(t *testing.T) {

	tests := []struct {
		name        string
		message     *pubsub.Message
		wantEventFn func() *cev2.Event
		wantErr     bool
	}{{
		name: "valid attributes",
		message: &pubsub.Message{
			ID:   "id",
			Data: []byte("test data"),
			Attributes: map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			},
		},
		wantEventFn: func() *cev2.Event {
			return pubSubPull(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			})
		},
	}, {
		name: "upper case attributes",
		message: &pubsub.Message{
			ID:   "id",
			Data: []byte("test data"),
			Attributes: map[string]string{
				"AttriBUte1": "value1",
				"AttrIbuTe2": "value2",
			},
		},
		wantEventFn: func() *cev2.Event {
			return pubSubPull(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			})
		},
	}, {
		name: "failing with invalid attributes",
		message: &pubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithProjectKey(context.Background(), "testproject")
			ctx = WithTopicKey(ctx, "testtopic")
			ctx = WithSubscriptionKey(ctx, "testsubscription")

			gotEvent, err := NewPubSubConverter().Convert(ctx, test.message, PubSubPull)
			if err != nil {
				if !test.wantErr {
					t.Errorf("converters.convertPubsubPull got error %v want error=%v", err, test.wantErr)
				}
			} else {
				if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
					t.Errorf("converters.convertPubsubPull got unexpeceted cloudevents.Event (-want +got) %s", diff)
				}
			}
		})
	}
}

func pubSubPull(extensions map[string]string) *cev2.Event {
	e := cev2.NewEvent(cev2.VersionV1)
	e.SetID("id")
	e.SetSource(schemasv1.CloudPubSubEventSource("testproject", "testtopic"))
	e.SetType(schemasv1.CloudPubSubMessagePublishedEventType)
	e.SetData("application/octet-stream", []byte("test data"))
	for k, v := range extensions {
		e.SetExtension(k, v)
	}
	return &e
}
