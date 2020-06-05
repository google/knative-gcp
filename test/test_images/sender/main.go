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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/test/e2e/lib"
	"go.opencensus.io/trace"
)

const (
	brokerURLEnvVar = "BROKER_URL"
)

func main() {
	brokerURL := os.Getenv(brokerURLEnvVar)

	ceClient, err := kncloudevents.NewDefaultClient(brokerURL)
	if err != nil {
		fmt.Printf("Unable to create ceClient: %s ", err)
	}

	ctx, span := trace.StartSpan(context.Background(), "sender", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	ctx, _, err = ceClient.Send(ctx, dummyCloudEvent())
	rtctx := cloudevents.HTTPTransportContextFrom(ctx)
	if err != nil {
		fmt.Print(err)
	}
	var success bool
	if rtctx.StatusCode >= http.StatusOK && rtctx.StatusCode < http.StatusBadRequest {
		success = true
	} else {
		success = false
	}
	if err := writeTerminationMessage(map[string]interface{}{
		"success": success,
		"traceid": span.SpanContext().TraceID.String(),
	}); err != nil {
		fmt.Printf("failed to write termination message, %s.\n", err)
	}
}

func dummyCloudEvent() cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(lib.E2EDummyEventID)
	event.SetType(lib.E2EDummyEventType)
	event.SetSource(lib.E2EDummyEventSource)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData(`{"source": "sender!"}`)
	return event
}

func writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
