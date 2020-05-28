package main

import (
	"fmt"
	"os"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/test_images/int/knockdown"
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
	fmt.Printf("SubjectPrefix to match: %q.\n", r.SubjectPrefix)
	fmt.Printf("Data to match: %q.\n", r.Data)
	fmt.Printf("Type to match: %q.\n", r.Type)

	return knockdown.Main(r.Config, r)
}

type schedulerReceiver struct {
	knockdown.Config

	SubjectPrefix string `envconfig:"SUBJECT_PREFIX" required:"true"`
	Data          string `envconfig:"DATA" required:"true"`
	Type          string `envconfig:"TYPE" required:"true"`
}

func (r *schedulerReceiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("scheduler target received event\n")
	fmt.Printf(event.Context.String())

	incorrectAttributes := make(map[string]lib.PropPair)

	// Check subject prefix
	subject := event.Subject()
	if !strings.HasPrefix(subject, r.SubjectPrefix) {
		incorrectAttributes[lib.EventSubjectPrefix] = lib.PropPair{Expected: r.SubjectPrefix, Received: subject}
	}

	// Check type
	evType := event.Type()
	if evType != r.Type {
		incorrectAttributes[lib.EventType] = lib.PropPair{Expected: r.Type, Received: evType}
	}

	// Check data
	data := string(event.Data.([]uint8))
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
