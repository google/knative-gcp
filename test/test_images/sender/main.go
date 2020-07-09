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
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	transport "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/test/e2e/lib"
)

const (
	brokerURLEnvVar = "BROKER_URL"
	retryEnvVar     = "RETRY"
)

// defaultRetry represents that there will be 3 iterations.
// The duration starts from 30s and is multiplied by factor 1.0 for each iteration.
var defaultRetry = wait.Backoff{
	Steps:    3,
	Duration: 30 * time.Second,
	Factor:   1.0,
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between 0 and jitter*duration.
	Jitter: 2.0,
}

func main() {
	brokerURL := os.Getenv(brokerURLEnvVar)
	retryVar := os.Getenv(retryEnvVar)

	needRetry := (retryVar == "true")

	ceClient, err := kncloudevents.NewDefaultClient(brokerURL)
	if err != nil {
		fmt.Printf("Unable to create ceClient: %s ", err)
	}

	ctx, span := trace.StartSpan(context.Background(), "sender", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	// If needRetry is true, repeat sending Event with exponential backoff when there are some specific errors.
	// In e2e test, sync problems could cause 404 and 5XX error, retrying those would help reduce flakiness.
	rtctx, err := sendEvent(ctx, ceClient, needRetry)
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

func sendEvent(ctx context.Context, ceClient cloudevents.Client, needRetry bool) (transport.TransportContext, error) {
	var rtctx transport.TransportContext
	send := func() error {
		ctx, _, err := ceClient.Send(ctx, dummyCloudEvent())
		rtctx = cloudevents.HTTPTransportContextFrom(ctx)
		return err
	}

	if needRetry {
		err := retry.OnError(defaultRetry, isRetryable, send)
		return rtctx, err
	}

	err := send()
	return rtctx, err
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

// isRetryable determines if the err is an error which is retryable
func isRetryable(err error) bool {
	return strings.Contains(err.Error(), "404 Not Found") || strings.Contains(err.Error(), "503 Service Unavailable") || strings.Contains(err.Error(), "500 Internal Server Error")
}
