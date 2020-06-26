/*
Copyright 2020 Google LLC
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
The Probe Helper implements the logic of a container image that converts asynchronous event delivery
to a synchronous call so that CEP can probe. It helps to measure the lantency and event success rate in SLO.
Concretely, the ProbeHelper is able to send and receive events. It exposes an HTTP endpoints which forwards
probe requests to the broker, and waits for the event to be delivered back to the it,
and returns the e2e latency as the response.
															4. (event)
							      ----------------------------------------------------
								  |													  |
								  v			 			       		         	      |
	Probe ---(event)-----> ProbeHelper ----(event)-----> Broker ------> trigger ------
				1.							2.					3. (blackbox)
*/
package main

import (
	"context"
	"crypto/rand"
	"log"
	"math"
	"math/big"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)
const (
	extensionName = "random-int"
	senderPort = 8070
	receiverPort = 8080
)

type cloudEventsFunc func(cloudevents.Event)

func forwardFromProbe(c cloudevents.Client, extension *big.Int, received chan bool) cloudEventsFunc{
	return func(event cloudevents.Event) {
		event.SetExtension(extensionName, extension)
		log.Print("Sending cloudevent to the broker")
		if res := c.Send(context.Background(), event); !cloudevents.IsACK(res) {
			log.Printf("Failed to send cloudevent: %v", res)
		}
		got := <- received
		if got {
			cloudevents.NewReceipt(true, "true")
		} else {
			cloudevents.NewReceipt(false, "false")
		}

	}
}

func receiveFromTrigger(extension *big.Int, received chan bool) cloudEventsFunc {
	return func(event cloudevents.Event) {
		gotExtension,exists := event.Extensions()[extensionName]
		if !exists || gotExtension != extension {
			received <- false
		} else {
			received <- true
		}
	}
}

func main() {
	brokerURL := os.Getenv("BROKER_URL")
	if len(brokerURL) == 0 {
		log.Fatalf("No broker url specified")
	}
	// create sender client
	sp, err := cloudevents.NewHTTP(cloudevents.WithPort(senderPort), cloudevents.WithTarget(brokerURL))
	if err != nil {
		log.Fatalf("Failed to create sender transport, %v", err)
	}
	sc, err  := cloudevents.NewClient(sp)
	if err != nil {
		log.Fatal("Failed to create sender client, ", err)
	}
	// create receiver client
	rp, err := cloudevents.NewHTTP(cloudevents.WithPort(receiverPort))
	if err != nil {
		log.Fatalf("Failed to create receiver transport, %v", err)
	}
	rc, err := cloudevents.NewClient(rp)
	if err != nil {
		log.Fatal("Failed to create receiver client, ", err)
	}
	// set up a random extension to be saved in the event as an identifier
	extension, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		log.Fatalf("Fail generate big int as an event extension id: %v", extension)
	}
	// make a channel for sync
	received := make(chan bool)
	// the goroutine to receive the event from probe and forward the event to the broker
	go func() {
		sc.StartReceiver(context.Background(), forwardFromProbe(sc, extension, received))
	}()
	// the goroutine to receive the event from the trigger and return the result back to the probe
	go func() {
		rc.StartReceiver(context.Background(), receiveFromTrigger(extension, received))
	}()
}
