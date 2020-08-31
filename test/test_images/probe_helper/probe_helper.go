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
	"encoding/json"
	"fmt"
	"log"
	nethttp "net/http"
	"sync"
	"time"

	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/kelseyhightower/envconfig"

	"knative.dev/pkg/signals"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
)

const (
	BrokerE2EDeliveryProbeEventType = "broker-e2e-delivery-probe"
	CloudPubSubSourceProbeEventType = "cloudpubsubsource-probe"

	maxStaleTime = 3 * time.Minute
)

var (
	lastSenderEventTimestamp   time.Time
	lastReceiverEventTimestamp time.Time
)

type cloudEventsFunc func(cloudevents.Event) protocol.Result

type receivedEventsMap struct {
	sync.RWMutex
	channels map[string]chan bool
}

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

func (r *receivedEventsMap) createReceiverChannel(event cloudevents.Event) (chan bool, error) {
	receiverChannel := make(chan bool, 1)
	r.Lock()
	defer r.Unlock()
	if _, ok := r.channels[event.ID()]; ok {
		return nil, fmt.Errorf("Receiver channel already exists for event %v", event.ID())
	}
	r.channels[event.ID()] = receiverChannel
	return receiverChannel, nil
}

func forwardFromProbe(ctx context.Context, brokerClient cloudevents.Client, pubsubClient cloudevents.Client, receivedEvents *receivedEventsMap, timeout int) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var err error
		var receiverChannel chan bool
		log.Printf("Received probe request: %+v \n", event)
		lastSenderEventTimestamp = time.Now()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
		defer cancel()
		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			receiverChannel, err = receivedEvents.createReceiverChannel(event)
			if err != nil {
				return cloudevents.NewReceipt(false, "Probe forwarding failed, could not create receiver channel: %v", err)
			}
			// The broker client forwards the event to the broker.
			if res := brokerClient.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when sending event %v to broker: %+v \n", event.ID(), res)
				return res
			}
		case CloudPubSubSourceProbeEventType:
			receiverChannel, err = receivedEvents.createReceiverChannel(event)
			if err != nil {
				return cloudevents.NewReceipt(false, "Probe forwarding failed, could not create receiver channel: %v", err)
			}
			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := pubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when publishing event %v to pubsub topic: %+v \n", event.ID(), res)
				return res
			}
		default:
			return cloudevents.NewReceipt(false, "Probe forwarding failed, unrecognized event type, %v", event.Type())
		}

		select {
		case <-receiverChannel:
			receivedEvents.Lock()
			delete(receivedEvents.channels, event.ID())
			receivedEvents.Unlock()
			return cloudevents.ResultACK
		case <-ctx.Done():
			log.Printf("Timed out waiting for the event to be sent back: %v \n", event.ID())
			return cloudevents.NewReceipt(false, "Timed out waiting for event to be sent back")
		}
	}
}

func receiveEvent(receivedEvents *receivedEventsMap) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var eventID string
		log.Printf("Received event: %+v \n", event)
		lastReceiverEventTimestamp = time.Now()

		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The event is received as sent.
			eventID = event.ID()
		case schemasv1.CloudPubSubMessagePublishedEventType:
			// The original event is wrapped into a pubsub Message by the CloudEvents
			// pubsub sender client, and encoded as data in a CloudEvent by the CloudPubSubSource.
			msgData := schemasv1.PushMessage{}
			if err := json.Unmarshal(event.Data(), &msgData); err != nil {
				log.Printf("Failed to unmarshal pubsub message data: %v", err)
				return cloudevents.ResultACK
			}
			var ok bool
			if eventID, ok = msgData.Message.Attributes["ce-id"]; !ok {
				log.Print("Failed to read CloudEvent ID from pubsub message")
				return cloudevents.ResultACK
			}
		default:
			log.Printf("Unrecognized event type: %v", event.Type())
			return cloudevents.ResultACK
		}

		receivedEvents.RLock()
		defer receivedEvents.RUnlock()
		receiver, ok := receivedEvents.channels[eventID]
		if !ok {
			log.Printf("This event is not received by the probe receiver client: %v \n", eventID)
			return cloudevents.ResultACK
		}
		receiver <- true
		return cloudevents.ResultACK
	}
}

func healthChecker(w nethttp.ResponseWriter, r *nethttp.Request) {
	now := time.Now()
	if (!lastSenderEventTimestamp.IsZero() && now.Sub(lastSenderEventTimestamp) > maxStaleTime) ||
		(!lastReceiverEventTimestamp.IsZero() && now.Sub(lastReceiverEventTimestamp) > maxStaleTime) {
		w.WriteHeader(nethttp.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(nethttp.StatusOK)
}

func runProbeHelper() {
	appcredentials.MustExistOrUnsetEnv()

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
		log.Fatal("Failed to create CloudEvents pubsub client, ", err)
	}

	// create sender client
	sp, err := cloudevents.NewHTTP(
		cloudevents.WithPort(probePort),
		cloudevents.WithTarget(brokerURL),
	)
	if err != nil {
		log.Fatalf("Failed to create sender transport, %v", err)
	}
	sc, err := cloudevents.NewClient(sp)
	if err != nil {
		log.Fatal("Failed to create sender client, ", err)
	}

	// create receiver client
	rp, err := cloudevents.NewHTTP(
		cloudevents.WithPort(receiverPort),
	)
	if err != nil {
		log.Fatalf("Failed to create receiver transport, %v", err)
	}
	rc, err := cloudevents.NewClient(rp)
	if err != nil {
		log.Fatal("Failed to create receiver client, ", err)
	}

	// start the health checker
	nethttp.HandleFunc("/healthz", healthChecker)
	go func() {
		log.Printf("Starting the health checker...")
		if err := nethttp.ListenAndServe(":8060", nil); err != nil && err != nethttp.ErrServerClosed {
			log.Printf("The health checker has stopped unexpectedly: %v", err)
		}
	}()

	// make a map to store the channel for each event
	receivedEvents := &receivedEventsMap{
		channels: make(map[string]chan bool),
	}
	// start a goroutine to receive the event from probe and forward it appropriately
	log.Println("Starting Probe Helper server...")
	go sc.StartReceiver(ctx, forwardFromProbe(ctx, sc, psc, receivedEvents, timeout))
	// Receive the event and return the result back to the probe
	log.Println("Starting event receiver...")
	rc.StartReceiver(ctx, receiveEvent(receivedEvents))
}
