package main

import (
	"fmt"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &receiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}

	return knockdown.Main(r.Config, r)
}

type receiver struct {
	knockdown.Config
}

func (r *receiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("target received event\n")
	fmt.Printf("context of event is: %v\n", event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	//Check ID
	if event.ID() != lib.E2EDummyRespEventID {
		incorrectAttributes[lib.EventID] = lib.PropPair{Expected: lib.E2EDummyRespEventID, Received: lib.E2EDummyRespEventID}
	}
	// Check type
	if event.Type() != lib.E2EDummyRespEventType {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: lib.E2EDummyRespEventType, Received: event.Type()}
	}

	// Check source
	if event.Source() != lib.E2EDummyRespEventSource {
		incorrectAttributes[lib.EventSource] = lib.PropPair{Expected: lib.E2EDummyRespEventType, Received: event.Source()}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Println(k, "expected:", v.Expected, "got:", v.Received)
	}
	return false
}
