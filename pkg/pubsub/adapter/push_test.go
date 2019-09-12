package adapter

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"

	cloudevents "github.com/cloudevents/sdk-go"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
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

	ctx := pubsubcontext.WithTransportContext(context.TODO(), pubsubcontext.NewTransportContext(
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

	ctx := pubsubcontext.WithTransportContext(context.TODO(), pubsubcontext.NewTransportContext(
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
