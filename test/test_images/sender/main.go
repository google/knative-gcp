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
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/e2e/lib"
	"go.opencensus.io/trace"
)

const (
	brokerURLEnvVar = "BROKER_URL"
)

func main() {
	brokerURL := os.Getenv(brokerURLEnvVar)

	t, err := cloudevents.NewHTTP(cloudevents.WithTarget(brokerURL))
	if err != nil {
		fmt.Printf("Unable to create transport: %v ", err)
		os.Exit(1)
	}

	c, err := cloudevents.NewClient(t,
		cloudevents.WithTimeNow(),
		cloudevents.WithUUIDs(),
	)

	if err != nil {
		fmt.Printf("Unable to create client: %v", err)
		os.Exit(1)
	}

	ctx, span := trace.StartSpan(context.Background(), "sender", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(lib.E2EDummyEventID)
	event.SetType(lib.E2EDummyEventType)
	event.SetSource(lib.E2EDummyEventSource)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData(cloudevents.ApplicationJSON, `{"source": "sender!"}`)

	var success bool
	if res := c.Send(ctx, event); !cloudevents.IsACK(res) {
		fmt.Printf("Failed to send event to %s: %s\n", brokerURL, res.Error())
		success = false
	} else {
		success = true
	}

	if err := writeTerminationMessage(map[string]interface{}{
		"success": success,
		"traceid": span.SpanContext().TraceID.String(),
	}); err != nil {
		fmt.Printf("failed to write termination message, %s.\n", err)
	}
}

func writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
