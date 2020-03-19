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
	"github.com/google/go-cmp/cmp"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	cepubsub "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub/context"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

func TestConvertCloudStorageSource(t *testing.T) {

	tests := []struct {
		name        string
		message     *cepubsub.Message
		sendMode    ModeType
		wantEventFn func() *cloudevents.Event
		wantErr     bool
	}{{
		name: "no attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{})
		},
		wantErr: true,
	}, {
		name: "no bucketId attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"eventType":   "OBJECT_FINALIZE",
				"attribute1":  "value1",
				"attribute2":  "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			})
		},
		wantErr: true,
	}, {
		name: "no eventType attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"bucketId":    "my-bucket",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{})
		},
		wantErr: true,
	}, {
		name: "set subject",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"bucketId":    "my-bucket",
				"eventType":   "OBJECT_FINALIZE",
				"objectId":    "myfile.jpg",
				"AttriBUte1":  "value1",
				"AttrIbuTe2":  "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			},
				"myfile.jpg")
		},
	}, {
		name: "not setting invalid upper case attributes",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"bucketId":    "my-bucket",
				"eventType":   "OBJECT_FINALIZE",
				"AttriBUte1":  "value1",
				"AttrIbuTe2":  "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{
				"attribute1": "value1",
				"attribute2": "value2",
			})
		},
	}, {
		name: "only setting valid alphanumeric attribute",
		message: &cepubsub.Message{
			Data: []byte("test data"),
			Attributes: map[string]string{
				"knative-gcp":       "com.google.cloud.storage",
				"bucketId":          "my-bucket",
				"eventType":         "OBJECT_FINALIZE",
				"attribute1":        "value1",
				"Invalid-Attrib#$^": "value2",
			},
		},
		sendMode: Binary,
		wantEventFn: func() *cloudevents.Event {
			return storageCloudEvent(map[string]string{
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

			gotEvent, err := Convert(ctx, test.message, test.sendMode, "")

			if err != nil {
				if !test.wantErr {
					t.Fatalf("converters.convertCloudStorage got error %v want error=%v", err, test.wantErr)
				}
			} else {
				if diff := cmp.Diff(test.wantEventFn(), gotEvent); diff != "" {
					t.Fatalf("converters.convertCloudStorage got unexpeceted cloudevents.Event (-want +got) %s", diff)
				}
			}
		})
	}
}

func storageCloudEvent(extensions map[string]string, subject ...string) *cloudevents.Event {
	e := cloudevents.NewEvent(cloudevents.VersionV1)
	e.SetID("id")
	e.SetDataContentType(*cloudevents.StringOfApplicationJSON())
	e.SetDataSchema(storageSchemaUrl)
	e.SetSource(v1alpha1.CloudStorageSourceEventSource("my-bucket"))
	e.SetType(v1alpha1.CloudStorageSourceFinalize)
	if len(subject) > 0 {
		e.SetSubject(subject[0])
	}
	e.Data = []byte("test data")
	e.DataEncoded = true
	for k, v := range extensions {
		e.SetExtension(k, v)
	}
	return &e
}
