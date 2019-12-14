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

package main

import (
	"context"
	"log"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go"
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
	// Check if the received event is the dummy event sent by sender pod.
	// If it is, send back a response CloudEvent.
	if event.ID() == "dummy" {
		resp.Status = http.StatusAccepted
		event = cloudevents.NewEvent(cloudevents.VersionV1)
		event.SetID("target")
		event.SetType("e2e-testing-resp")
		event.SetSource("e2e-testing")
		event.SetDataContentType(cloudevents.ApplicationJSON)
		event.SetData(`{"hello": "world!"}`)
		event.SetExtension("target", "falldown")
		resp.Event = &event
	} else {
		resp.Status = http.StatusForbidden
	}
}
