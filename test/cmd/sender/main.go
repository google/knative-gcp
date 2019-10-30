package main

import (
	"context"
	"encoding/json"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	"io/ioutil"
	"net/http"
	"os"
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
	event := dummyCloudEvent()
	rctx, _, err := ceClient.Send(context.TODO(), event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	if err != nil {
		fmt.Printf(err.Error())
	}
	fmt.Printf(rtctx.String())
	var success bool
	if rtctx.StatusCode >= http.StatusOK && rtctx.StatusCode < http.StatusBadRequest {
		success = true
	} else {
		success = false
	}
	if err := writeTerminationMessage(map[string]interface{}{
		"success": success,
	}); err != nil {
		fmt.Printf("failed to write termination message, %s.\n", err)
	}

	os.Exit(0)
}

func dummyCloudEvent() cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID("dummy")
	event.SetType("e2e-testing-dummy")
	event.SetSource("e2e-testing")
	event.SetDataContentType("application/json")
	return event
}

func writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
