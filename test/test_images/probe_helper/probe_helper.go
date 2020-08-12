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

	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/kelseyhightower/envconfig"

	"knative.dev/pkg/signals"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/utils"
)

type cloudEventsFunc func(cloudevents.Event) protocol.Result

type envConfig struct {
	// Environment variable containing the project ID
	ProjectID string `envconfig:"PROJECT_ID"`

	// Environment variable containing the sink URL (broker URL) that the event will be forwarded to by the probeHelper for the e2e delivery probe
	BrokerURL string `envconfig:"K_SINK" default:"http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-probe/default"`

	// Environment variable containing the CloudPubSubSource Topic ID that the event will be forwarded to by the probeHelper for the CloudPubSubSource probe
	CloudPubSubSourceTopicID string `envconfig:"CLOUDPUBSUBSOURCE_TOPIC_ID" default:"cloudpubsubsource-topic"`

	// Environment variable containing the port which listens to the probe to deliver the event
	ProbePort int `envconfig:"PROBE_PORT" default:"8070"`

	// Environment variable containing the port to receive the event from the trigger
	ReceiverPort int `envconfig:"RECEIVER_PORT" default:"8080"`

	// Environment variable containing the timeout period to wait for an event to be delivered back (in minutes)
	Timeout int `envconfig:"TIMEOUT_MINS" default:"30"`
}

func forwardFromProbe(ctx context.Context, sc cloudevents.Client, psc cloudevents.Client, receivedEvents map[string]chan bool, timeout int) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		log.Printf("Received probe request: %+v \n", event)
		eventID := event.ID()
		receivedEvents[eventID] = make(chan bool, 1)
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
		defer cancel()
		switch event.Type() {
		case "broker-e2e-delivery-probe", "broker-e2e-delivery-probe-kubectl-exec", "broker-e2e-delivery-probe-kubectl-exec-warm-up":
			// The sender client forwards the event to the broker.
			if res := sc.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when sending event %v to broker: %+v \n", eventID, res)
				return res
			}
		case "cloudpubsubsource-probe", "cloudpubsubsource-probe-kubectl-exec", "cloudpubsubsource-probe-kubectl-exec-warm-up":
			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := psc.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when publishing event %v to pubsub topic: %+v \n", eventID, res)
				return res
			}
		default:
			return cloudevents.NewReceipt(false, "probe forwarding failed, unrecognized event type, %v", event.Type())
		}
		select {
		case <-receivedEvents[eventID]:
			delete(receivedEvents, eventID)
			return cloudevents.ResultACK
		case <-ctx.Done():
			log.Printf("Timed out waiting for the event to be sent back: %v \n", eventID)
			return cloudevents.NewReceipt(false, "timed out waiting for event to be sent back")
		}
	}
}

func receiveFromTrigger(receivedEvents map[string]chan bool) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		log.Printf("Received event: %+v \n", event)
		eventID := event.ID()
		ch, ok := receivedEvents[eventID]
		if !ok {
			log.Printf("This event is not received by the probe receiver client: %v \n", eventID)
			return cloudevents.ResultACK
		}
		ch <- true
		return cloudevents.ResultACK
	}
}

func runProbeHelper() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var, %v", err)
	}
	projectID, err := utils.ProjectID(env.ProjectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		log.Fatalf("Failed to get the default project ID, %v", err)
	}
	brokerURL := env.BrokerURL
	probePort := env.ProbePort
	receiverPort := env.ReceiverPort
	timeout := env.Timeout
	ctx := signals.NewContext()
	log.Printf("Running Probe Helper with env config: %+v \n", env)
	// create pubsub client
	pst, err := cepubsub.New(ctx,
		cepubsub.WithProjectID(projectID),
		cepubsub.WithTopicID(env.CloudPubSubSourceTopicID))
	if err != nil {
		log.Fatalf("Failed to create pubsub transport, %v", err)
	}
	psc, err := cloudevents.NewClient(pst)
	if err != nil {
		log.Fatal("Failed to create pubsub client, ", err)
	}
	// create sender client
	sp, err := cloudevents.NewHTTP(cloudevents.WithPort(probePort), cloudevents.WithTarget(brokerURL))
	if err != nil {
		log.Fatalf("Failed to create sender transport, %v", err)
	}
	sc, err := cloudevents.NewClient(sp)
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
	// make a map to store the channel for each event
	receivedEvents := make(map[string]chan bool)
	// start a goroutine to receive the event from probe and forward it appropriately
	log.Println("Starting Probe Helper server...")
	go sc.StartReceiver(ctx, forwardFromProbe(ctx, sc, psc, receivedEvents, timeout))
	// Receive the event from a trigger and return the result back to the probe
	log.Println("Starting event receiver...")
	rc.StartReceiver(ctx, receiveFromTrigger(receivedEvents))
}
