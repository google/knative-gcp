package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/kelseyhightower/envconfig"
)

const (
	eventSubject = "subject"
	eventData    = "data"
	eventType    = "type"
)

var shutDown = make(chan struct{}, 1)

func main() {
	client, err := cloudevents.NewDefaultClient()
	if err != nil {
		panic(err)
	}

	r := Receiver{}
	if err := envconfig.Process("", &r); err != nil {
		panic(err)
	}

	fmt.Printf("Waiting to receive event (timeout in %s seconds)...", r.Time)

	duration, _ := strconv.Atoi(r.Time)
	timer := time.NewTimer(time.Second * time.Duration(duration))
	defer timer.Stop()

	go func() {
		select {
		case <-shutDown:
			// Give the receiver a little time to finish responding.
			time.Sleep(time.Second)
			os.Exit(0)
		case <-timer.C:
			// Write the termination message if time out occurred
			fmt.Println("Timed out waiting for event from scheduler")
			if err := r.writeTerminationMessage(map[string]interface{}{
				"shutDown": false,
			}); err != nil {
				fmt.Println("Failed to write termination message, got error:", err.Error())
			}
			os.Exit(0)
		}
	}()

	if err := client.StartReceiver(context.Background(), r.Receive); err != nil {
		log.Fatal(err)
	}
}

type Receiver struct {
	Time          string `envconfig:"TIME" required:"true"`
	SubjectPrefix string `envconfig:"SUBJECT_PREFIX" required:"true"`
	Data          string `envconfig:"DATA" required:"true"`
	EventType     string `envconfig:"TYPE" required:"true"`
}

type propPair struct {
	expected string
	received string
}

func (r *Receiver) Receive(event cloudevents.Event) {
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
		// Write the termination message.
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": true,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err)
		}
	} else {
		if err := r.writeTerminationMessage(map[string]interface{}{
			"success": false,
		}); err != nil {
			fmt.Printf("failed to write termination message, %s.\n", err)
		}
		for k, v := range incorrectAttributes {
			if k == eventSubject {
				fmt.Println(v.received, "did not have expected prefix", v.expected)
			} else {
				fmt.Println(k, "expected:", v.expected, "got:", v.received)
			}
		}
	}
	shutDown <- struct{}{}
}

func (r *Receiver) writeTerminationMessage(result interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/dev/termination-log", b, 0644)
}
