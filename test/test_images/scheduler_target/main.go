package main

import (
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/test/test_images/internal/knockdown"
	"github.com/kelseyhightower/envconfig"
)

const (
	eventSubject = "subject"
	eventData    = "data"
	eventType    = "type"
)

func main() {
	r := &Receiver{}
	if err := envconfig.Process("", &r); err != nil {
		panic(err)
	}

	knockdown.Main(r.Config, r)
}

type Receiver struct {
	knockdown.Config

	SubjectPrefix string `envconfig:"SUBJECT_PREFIX" required:"true"`
	Data          string `envconfig:"DATA" required:"true"`
	EventType     string `envconfig:"TYPE" required:"true"`
}

type propPair struct {
	expected string
	received string
}

func (r *Receiver) Knockdown(event cloudevents.Event) bool {
	// Print out event received to log
	fmt.Printf("Received event\n")
	fmt.Printf("	Context: %v\n", event.Context.String())
	fmt.Printf("	Data: %s\n", event.Data)
	fmt.Printf("	Encoded: %v\n", event.DataEncoded)
	fmt.Printf("	Binary: %v\n", event.DataBinary)

	incorrectAttributes := make(map[string]propPair)

	// Check subject prefix
	subject := event.Subject()
	if !strings.HasPrefix(subject, r.SubjectPrefix) {
		incorrectAttributes[eventSubject] = propPair{r.SubjectPrefix, subject}
	}

	// Check type
	evType := event.Type()
	if evType != r.EventType {
		incorrectAttributes[eventType] = propPair{r.EventType, evType}
	}

	// Check data
	data := string(event.Data.([]uint8))
	if data != r.Data {
		incorrectAttributes[eventData] = propPair{r.Data, data}
	}

	if len(incorrectAttributes) == 0 {
		return true
	}
	for k, v := range incorrectAttributes {
		if k == eventSubject {
			fmt.Println(v.received, "did not have expected prefix", v.expected)
		} else {
			fmt.Println(k, "expected:", v.expected, "got:", v.received)
		}
	}
	return false
}
