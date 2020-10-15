/*
Copyright 2020 Google LLC

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
	"fmt"
	"log"
	"net/http"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/lib"
)

type Receiver struct {
	client cloudevents.Client
}

func main() {
	client, err := kncloudevents.NewDefaultClient()
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

func (r *Receiver) Receive(ctx context.Context, event cloudevents.Event) (*event.Event, protocol.Result) {
	// Check if the received event is the event sent by CloudSchedulerSource.
	// If it is, send back a response CloudEvent.
	// Print out event received to log
	fmt.Printf("scheduler receiver received event\n")
	fmt.Printf("context of event is: %v\n", event.Context.String())

	if event.Type() == schemasv1.CloudSchedulerJobExecutedEventType {
		respEvent := cloudevents.NewEvent(cloudevents.VersionV1)
		respEvent.SetID(lib.E2ESchedulerRespEventID)
		respEvent.SetType(lib.E2ESchedulerRespType)
		respEvent.SetSource(event.Source())
		respEvent.SetSubject(event.Subject())
		respEvent.SetData(event.DataContentType(), event.Data())
		fmt.Printf("context of respEvent is: %v\n", respEvent.Context.String())
		return &respEvent, cehttp.NewResult(http.StatusAccepted, "OK")
	} else {
		return nil, cehttp.NewResult(http.StatusForbidden, "Forbidden")
	}
}
