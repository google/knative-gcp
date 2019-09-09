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

package adapter

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
)

func TestConvertToPush_noattrs(t *testing.T) {
	want := `Validation: valid
Context Attributes,
  specversion: 0.2
  type: unit.testing
  source: source
  id: abc-123
  contenttype: application/json
Data,
  {
    "subscription": "sub",
    "message": {
      "id": "abc-123",
      "data": "testing",
      "publish_time": "0001-01-01T00:00:00Z"
    }
  }
`

	event := cloudevents.NewEvent()
	event.SetSource("source")
	event.SetType("unit.testing")
	event.SetID("abc-123")
	event.SetDataContentType("application/json")
	_ = event.SetData("testing")

	ctx := cepubsub.WithTransportContext(context.TODO(), cepubsub.NewTransportContext(
		"proj",
		"top",
		"sub",
		"test",
		&pubsub.Message{},
	))

	got := ConvertToPush(ctx, event)

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Logf("%s", got.String())
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}

func TestConvertToPush_attrs(t *testing.T) {
	want := `Validation: valid
Context Attributes,
  specversion: 0.2
  type: unit.testing
  source: source
  id: abc-123
  contenttype: application/json
Extensions,
  foo: bar
Data,
  {
    "subscription": "sub",
    "message": {
      "id": "abc-123",
      "data": "testing",
      "attributes": {
        "foo": "bar"
      },
      "publish_time": "0001-01-01T00:00:00Z"
    }
  }
`

	event := cloudevents.NewEvent()
	event.SetSource("source")
	event.SetType("unit.testing")
	event.SetID("abc-123")
	event.SetExtension("foo", "bar")
	event.SetDataContentType("application/json")
	_ = event.SetData("testing")

	ctx := cepubsub.WithTransportContext(context.TODO(), cepubsub.NewTransportContext(
		"proj",
		"top",
		"sub",
		"test",
		&pubsub.Message{},
	))

	got := ConvertToPush(ctx, event)

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Logf("%s", got.String())
		t.Errorf("failed to get expected (-want, +got) = %v", diff)
	}
}
