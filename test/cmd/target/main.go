package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
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
	if err := event.ExtensionAs("target", &target); err != nil {
		fmt.Println("failed to get target from extensions:", err)
		return
	}
	var success bool
	if strings.HasPrefix(r.Target, target) {
		fmt.Printf("Target prefix matched, %q.\n", r.Target)
		success = true
	} else {
		fmt.Printf("Target prefix did not match, %q != %q.\n", target, r.Target)
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
