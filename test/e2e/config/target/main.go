package main

import (
	"context"
	"fmt"
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
	if strings.HasPrefix(r.Target, target) {
		fmt.Printf("Target prefix matched, %q.\n", r.Target)
		os.Exit(0)
	} else {
		fmt.Printf("Target prefix did not match, %q != %q.\n", target, r.Target)
	}
}
