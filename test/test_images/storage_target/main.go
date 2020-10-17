package main

import (
	"fmt"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/lib"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &storageReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}

	fmt.Printf("Type to match: %q.\n", r.Type)
	fmt.Printf("Source to match: %q.\n", r.Source)
	fmt.Printf("Subject to match: %q.\n", r.Subject)
	return knockdown.Main(r.Config, r)
}

type storageReceiver struct {
	knockdown.Config

	Type    string `envconfig:"TYPE" required:"true"`
	Source  string `envconfig:"SOURCE" required:"true"`
	Subject string `envconfig:"SUBJECT" required:"true"`
}

func (r *storageReceiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("storage target received event\n")
	fmt.Print(event.Context)

	incorrectAttributes := make(map[string]lib.PropPair)

	// Check type
	if event.Type() != r.Type {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: r.Type, Received: event.Type()}
	}

	// Check source
	if event.Source() != r.Source {
		incorrectAttributes[lib.EventSource] = lib.PropPair{Expected: r.Source, Received: event.Source()}
	}

	// Check subject
	if event.Subject() != r.Subject {
		incorrectAttributes[lib.EventSubject] = lib.PropPair{Expected: r.Subject, Received: event.Subject()}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		fmt.Println(k, "expected:", v.Expected, "got:", v.Received)
	}
	return false
}
