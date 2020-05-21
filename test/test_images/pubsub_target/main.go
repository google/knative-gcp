package main

import (
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
	"os"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &pubsubReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}
	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Source to match: %q.\n", r.Source)
	return knockdown.Main(r.Config, r)
}

type pubsubReceiver struct {
	knockdown.Config
	Type   string `envconfig:"TYPE" required:"true"`
	Source string `envconfig:"SOURCE" required:"true"`
}

func (r *pubsubReceiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	// Print out event received to log
	fmt.Printf("pubsub target received event\n")
	fmt.Printf("context of event is: %v\n", event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	// Check type
	if event.Type() != r.Type {
		incorrectAttributes[r.Type] = lib.PropPair{Expected: r.Type, Received: event.Type()}
	}

	// Check source
	if event.Source() != r.Source {
		incorrectAttributes[lib.EventSource] = lib.PropPair{Expected: r.Source, Received: event.Source()}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Println(k, "expected:", v.Expected, "got:", v.Received)
	}
	return false
}
