package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/cloudevents/sdk-go"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	r := Receiver{}
	if err := envconfig.Process("", &r); err != nil {
		panic(err)
	}

	fmt.Printf("Target prefix to match: %q.\n", r.Target)

	if err := client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

type Receiver struct {
	Target string `envconfig:"TARGET" required:"true"`
}

func (r *Receiver) Receive(event cloudevents.Event) {
	var target string

	// Try Pull first.
	err := event.ExtensionAs("target", &target)
	if err != nil {
		// Should be Push format and the target should be in the data payload.
		data, err := event.DataBytes()
		if err != nil {
			fmt.Println("failed to get data from event", err)
			return
		}
		msg := converters.PubSubMessage{}
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Println("failed to unmarshall PubSubMessage", err)
			return
		}

		if tt, ok := msg.Attributes["target"]; !ok {
			fmt.Println("failed to get target from attributes:", err)
			return
		} else {
			target = tt
		}
	}

	var success bool
	if strings.Contains(r.Target, target) {
		fmt.Printf("Target found, %q.\n", r.Target)
		success = true
	} else {
		fmt.Printf("Target not found, got:%q, want:%q.\n", target, r.Target)
		success = false
	}
	// Write the termination message.
	if err := r.writeTerminationMessage(map[string]interface{}{
		"success": success,
	}); err != nil {
		fmt.Printf("failed to write termination message, %s.\n", err)
	}
	os.Exit(0)
}

func (r *Receiver) writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
