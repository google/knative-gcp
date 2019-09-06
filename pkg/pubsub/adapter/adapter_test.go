package adapter

import (
	"context"
	"knative.dev/pkg/metrics"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go"
	. "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
)

type mockStatsReporter struct{}

// TODO: test this more.
func (r *mockStatsReporter) ReportEventCount(args *metrics.ReportArgs, responseCode int) error {
	return nil
}

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

	msg := &Message{}
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
	r := &mockStatsReporter{}
	a := Adapter{
		Project:      "proj",
		Topic:        "top",
		Subscription: "sub",
		SendMode:     "binary",

		outbound: c,
		reporter: r,
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
