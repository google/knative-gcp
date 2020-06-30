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

package main

import (
	"context"
	"log"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/kelseyhightower/envconfig"
)

type cloudEventsFunc func(cloudevents.Event) protocol.Result

type envConfig struct {
	// Environment variable containing the sink URL (broker URL) that the event will be forwarded to by the probeHelper
	BrokerURL string `envconfig:"K_SINK" default:"http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-probe/default"`

	// Environment variable containing the port which listens to the probe to deliver the event
	ProbePort int `envconfig:"PROBE_PORT" default:"8070"`

	// Environment variable containing the port to receive the event from the trigger
	ReceiverPort int `envconfig:"RECEIVER_PORT" default:"8080"`

	// Environment variable containing the timeout period to wait for an event to be delivered back (in seconds)
	Timeout int `envconfig:"TIMEOUT" default:"300"`
}

func forwardFromProbe(c cloudevents.Client, receivedEvents map[string] chan bool, timeout int) cloudEventsFunc{
	return func(event cloudevents.Event) protocol.Result{
		eventID := event.ID()
		log.Print("Sending the cloud event to the broker")
		receivedEvents[eventID] = make(chan bool, 1)
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
		if res := c.Send(ctx, event); !cloudevents.IsACK(res) {
			log.Fatalf("Failed to send cloudevent: %v", res)
		}
		select {
		case <-receivedEvents[eventID]:
			delete(receivedEvents, eventID)
			return cloudevents.ResultACK
		case <-ctx.Done():
			return cloudevents.ResultNACK
		}
	}
}

func receiveFromTrigger(receivedEvents map[string]chan bool) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		eventID := event.ID()
		receivedEvents[eventID] <- true
		return cloudevents.ResultACK
	}
}

func runProbeHelper() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var, %v", err)
	}
	brokerURL := env.BrokerURL
	probePort := env.ProbePort
	receiverPort := env.ReceiverPort
	timeout := env.Timeout
	// create sender client
	sp, err := cloudevents.NewHTTP(cloudevents.WithPort(probePort), cloudevents.WithTarget(brokerURL))
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
	// make a channel for sync
	receivedEvents := make(map[string]chan bool)
	// the goroutine to receive the event from probe and forward the event to the broker
	go sc.StartReceiver(context.Background(), forwardFromProbe(sc, receivedEvents, timeout))
	// the goroutine to receive the event from the trigger and return the result back to the probe
	go rc.StartReceiver(context.Background(), receiveFromTrigger(receivedEvents))
}
