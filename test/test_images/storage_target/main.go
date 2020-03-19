package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v1"
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

	fmt.Printf("Subject to match: %q.\n", r.Subject)

	// Create a timer
	duration, _ := strconv.Atoi(r.Time)
	timer := time.NewTimer(time.Second * time.Duration(duration))
	defer timer.Stop()
	go func() {
		<-timer.C
		// Write the termination message if time out
		fmt.Printf("time out to wait for event with subject %q.\n", r.Subject)
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": false,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err.Error())
		}
		os.Exit(0)
	}()

	if err := client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

type Receiver struct {
	Subject string `envconfig:"SUBJECT" required:"true"`
	Time    string `envconfig:"TIME" required:"true"`
}

func (r *Receiver) Receive(event cloudevents.Event) {
	eventSubject := event.Context.GetSubject()
	fmt.Printf(event.Context.String())
	if eventSubject == r.Subject {
		fmt.Printf("subject matches, %q.\n", r.Subject)
		// Write the termination message if the subject successfully matches
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": true,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err.Error())
		}
		os.Exit(0)
	} else {
		fmt.Printf("subject doesn't match, %q != %q.\n", eventSubject, r.Subject)
	}
}

func (r *Receiver) writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
