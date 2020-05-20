package main

import (
	"fmt"
	"os"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	r := &Receiver{}
	if err := envconfig.Process("", r); err != nil {
		panic(err)
	}

	return knockdown.Main(r.Config, r)
}

type Receiver struct {
	knockdown.Config

	Subject string `envconfig:"SUBJECT" required:"true"`
}

func (r *Receiver) Knockdown(event cloudevents.Event) bool {
	eventSubject := event.Context.GetSubject()
	fmt.Printf(event.Context.String())
	if eventSubject == r.Subject {
		return true
	}
	return false
}
