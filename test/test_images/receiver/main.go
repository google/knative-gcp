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
	"os"
	"strconv"
	"sync"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/test/lib"
)

const (
	firstNErrsEnvVar = "FIRST_N_ERRS"
)

type Receiver struct {
	client     cloudevents.Client
	errsCount  int
	firstNErrs int
	mux        sync.Mutex
}

func main() {
	client, err := kncloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	firstNErrsStr := os.Getenv(firstNErrsEnvVar)
	firstNErrs := 0
	if firstNErrsStr != "" {
		var err error
		firstNErrs, err = strconv.Atoi(firstNErrsStr)
		if err != nil {
			panic(err)
		}
	}

	r := &Receiver{
		client:     client,
		errsCount:  0,
		firstNErrs: firstNErrs,
	}
	if err := r.client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

func (r *Receiver) Receive(ctx context.Context, event cloudevents.Event) (*event.Event, protocol.Result) {
	if r.shouldReturnErr() {
		// Ksvc seems to auto retry 5xx. So use 4xx for predictability.
		return nil, cehttp.NewResult(http.StatusBadRequest, "Seeding failure receiver response with 400")
	}

	// Check if the received event is the sample event sent by sender pod.
	// If it is, send back a response CloudEvent.
	if event.ID() == lib.E2ESampleEventID {
		event = cloudevents.NewEvent(cloudevents.VersionV1)
		event.SetID(lib.E2ESampleRespEventID)
		event.SetType(lib.E2ESampleRespEventType)
		event.SetSource(lib.E2ESampleRespEventSource)
		event.SetData(cloudevents.ApplicationJSON, `{"source": "receiver!"}`)
		return &event, cehttp.NewResult(http.StatusAccepted, "OK")
	} else {
		return nil, cehttp.NewResult(http.StatusForbidden, "Forbidden")
	}
}

func (r *Receiver) shouldReturnErr() bool {
	r.mux.Lock()
	defer r.mux.Unlock()
	if r.errsCount >= r.firstNErrs {
		return false
	}
	r.errsCount++
	return true
}
