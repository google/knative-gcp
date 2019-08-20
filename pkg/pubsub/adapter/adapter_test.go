package adapter

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"

	"cloud.google.com/go/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
)

// TODO: test this more.
func TestAdapter(t *testing.T) {
	ctx := context.TODO()
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
	}

	_ = a.Start(ctx)
}

// TODO: test this more.
func TestConvert(t *testing.T) {
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
	}

	ctx := cepubsub.WithTransportContext(context.TODO(), cepubsub.NewTransportContext(
		"proj",
		"top",
		"sub",
		"test",
		&pubsub.Message{
			ID: "abc",
			Attributes: map[string]string{
				"test": "me",
			},
		},
	))

	msg := &cepubsub.Message{}
	var err error
	event, gotErr := a.convert(ctx, msg, err)

	_ = event
	_ = gotErr

}

// TODO: test this more.
func TestHTTPClient(t *testing.T) {
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
		SendMode:     "binary",
	}

	_, _ = a.newHTTPClient(context.TODO(), "http://localhost:8080")
}

// TODO: test this more.
func TestReceive(t *testing.T) {
	c, _ := cloudevents.NewDefaultClient()
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
		SendMode:     "binary",

		outbound: c,
	}

	ctx := context.Background()
	event := cloudevents.NewEvent()
	event.SetSource("source")
	event.SetType("unit.testing")
	event.SetID("abc-123")
	event.SetDataContentType("application/json")
	_ = event.SetData("testing")

	_ = a.receive(ctx, event, nil)
}
