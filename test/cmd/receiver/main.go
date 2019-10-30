package main

import (
	"context"
	cloudevents "github.com/cloudevents/sdk-go"
	"log"
	"net/http"
)

type Receiver struct {
	client cloudevents.Client
}

func main() {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}
	r := &Receiver{
		client: client,
	}
	if err := r.client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

func (r *Receiver) Receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) {
	if event.ID() == "dummy" {
		resp.Status = http.StatusAccepted
		event = cloudevents.NewEvent()
		resp.Event = &event
		resp.Event.SetID("target")
		resp.Event.SetType("e2e-testing-resp")
		resp.Event.SetSource("e2e-testing")
		resp.Event.SetDataContentType("application/json")
		resp.Event.SetData(`{"hello": "world!"}`)
		resp.Event.SetExtension("target", "falldown")
	} else {
		resp.Status = http.StatusForbidden
	}
}
