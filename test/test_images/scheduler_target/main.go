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
	r := &schedulerReceiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}
	fmt.Printf("Data to match: %q.\n", r.Data)
	fmt.Printf("Type to match: %q.\n", r.Type)

	return knockdown.Main(r.Config, r)
}

type schedulerReceiver struct {
	knockdown.Config

	Data string `envconfig:"DATA" required:"true"`
	Type string `envconfig:"TYPE" required:"true"`
}

func (r *schedulerReceiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("scheduler target received event\n")
	fmt.Printf(event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	// Check type
	evType := event.Type()
	if evType != r.Type {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: r.Type, Received: evType}
	}

	// Check data
	// TODO fix!
	data := string(event.Data())
	if data != r.Data {
		incorrectAttributes[lib.EventData] = lib.PropPair{Expected: r.Data, Received: data}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		if k == lib.EventSubjectPrefix {
			fmt.Println(v.Received, "did not have expected prefix", v.Expected)
		} else {
			fmt.Println(k, "expected:", v.Expected, "got:", v.Received)
		}
	}
	return false
}
