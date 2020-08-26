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
	"sync"
	"time"

	"cloud.google.com/go/storage"
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
	BrokerE2EDeliveryProbeEventType              = "broker-e2e-delivery-probe"
	CloudPubSubSourceProbeEventType              = "cloudpubsubsource-probe"
	CloudStorageSourceProbeEventType             = "cloudstoragesource-probe"
	CloudStorageSourceProbeCreateSubject         = "create"
	CloudStorageSourceProbeUpdateMetadataSubject = "update-metadata"
	CloudStorageSourceProbeArchiveSubject        = "archive"
	CloudStorageSourceProbeDeleteSubject         = "delete"
	CloudSchedulerSourceProbeEventType           = "cloudschedulersource-probe"

	cloudSchedulerSourceProbeChannelID = "cloudschedulersource-probe-channel-id"
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

	// Environment variable containing the CloudStorageSource Bucket ID that objects will be written to by the probeHelper for the CloudStorageSource probe
	CloudStorageSourceBucketID string `envconfig:"CLOUDSTORAGESOURCE_BUCKET_ID" default:"project-id-cloudstoragesource-bucket"`

	// Environment variable containing the port which listens to the probe to deliver the event
	ProbePort int `envconfig:"PROBE_PORT" default:"8070"`

	// Environment variable containing the port to receive the event from the trigger
	ReceiverPort int `envconfig:"RECEIVER_PORT" default:"8080"`

	// Environment variable containing the timeout period to wait for an event to be delivered back (in minutes)
	Timeout int `envconfig:"TIMEOUT_MINS" default:"30"`
}

func (r *receivedEventsMap) createReceiverChannel(channelID string) (chan bool, error) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.channels[channelID]; ok {
		return nil, fmt.Errorf("Receiver channel already exists for key %v", channelID)
	}
	receiverChannel := make(chan bool, 1)
	r.channels[channelID] = receiverChannel
	return receiverChannel, nil
}

func (r *receivedEventsMap) deleteReceiverChannel(channelID string) {
	r.Lock()
	if ch, ok := r.channels[channelID]; ok {
		close(ch)
		delete(r.channels, channelID)
	}
	r.Unlock()
}

func forwardFromProbe(ctx context.Context, brokerClient cloudevents.Client, pubsubClient cloudevents.Client, bucket *storage.BucketHandle, receivedEvents *receivedEventsMap, timeout int) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string
		var err error
		var receiverChannel chan bool
		log.Printf("Received probe request: %+v \n", event)

		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
		defer cancel()
		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			channelID = event.ID()
			receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
				return cloudevents.ResultNACK
			}
			defer receivedEvents.deleteReceiverChannel(channelID)

			// The broker client forwards the event to the broker.
			if res := brokerClient.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when sending event %v to broker: %+v \n", event.ID(), res)
				return res
			}
		case CloudPubSubSourceProbeEventType:
			channelID = event.ID()
			receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
				return cloudevents.ResultNACK
			}
			defer receivedEvents.deleteReceiverChannel(channelID)

			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := pubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
				log.Printf("Error when publishing event %v to pubsub topic: %+v \n", event.ID(), res)
				return res
			}
		case CloudStorageSourceProbeEventType:
			obj := bucket.Object(event.ID())
			switch event.Subject() {
			case CloudStorageSourceProbeCreateSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
					return cloudevents.ResultNACK
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// The storage client writes an object named as the event ID.
				w := obj.NewWriter(ctx)
				if _, err := fmt.Fprintf(w, event.String()); err != nil {
					log.Printf("Probe forwarding failed, error writing object %v to bucket: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
				if err := w.Close(); err != nil {
					log.Printf("Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
			case CloudStorageSourceProbeUpdateMetadataSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
					return cloudevents.ResultNACK
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// The storage client updates the object metadata.
				objectAttrs := storage.ObjectAttrsToUpdate{
					Metadata: map[string]string{
						"some-key": "Metadata updated!",
					},
				}
				if _, err := obj.Update(ctx, objectAttrs); err != nil {
					log.Printf("Probe forwarding failed, could not update metadata for object %v: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
			case CloudStorageSourceProbeArchiveSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
					return cloudevents.ResultNACK
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// The storage client updates the object's storage class to ARCHIVE.
				w := obj.NewWriter(ctx)
				w.ObjectAttrs.StorageClass = "ARCHIVE"
				if _, err := fmt.Fprintf(w, event.String()); err != nil {
					log.Printf("Probe forwarding failed, error writing object %v to bucket: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
				if err := w.Close(); err != nil {
					log.Printf("Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
			case CloudStorageSourceProbeDeleteSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
					return cloudevents.ResultNACK
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// Deleting a specific version of an object deletes it forever.
				objectAttrs, err := obj.Attrs(ctx)
				if err != nil {
					log.Printf("Probe forwarding failed, error getting attributes for object %v: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
				if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
					log.Printf("Probe forwarding failed, error deleting object %v: %v", event.ID(), err)
					return cloudevents.ResultNACK
				}
			default:
				log.Printf("Probe forwarding failed, unrecognized cloud storage probe subject: %v", event.Subject())
				return cloudevents.ResultNACK
			}
		case CloudSchedulerSourceProbeEventType:
			channelID = cloudSchedulerSourceProbeChannelID
			receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				log.Printf("Probe forwarding failed, could not create receiver channel: %v", err)
				return cloudevents.ResultNACK
			}
			defer receivedEvents.deleteReceiverChannel(channelID)
		default:
			log.Printf("Probe forwarding failed, unrecognized event type, %v", event.Type())
			return cloudevents.ResultNACK
		}

		select {
		case <-receiverChannel:
			return cloudevents.ResultACK
		case <-ctx.Done():
			log.Printf("Timed out waiting for the receiver channel: %v \n", channelID)
			return cloudevents.ResultNACK
		}
	}
}

func receiveEvent(receivedEvents *receivedEventsMap) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string

		log.Printf("Received event: %+v \n", event)
		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The event is received as sent.
			channelID = event.ID()
		case schemasv1.CloudPubSubMessagePublishedEventType:
			// The original event is wrapped into a pubsub Message by the CloudEvents
			// pubsub sender client, and encoded as data in a CloudEvent by the CloudPubSubSource.
			msgData := schemasv1.PushMessage{}
			if err := json.Unmarshal(event.Data(), &msgData); err != nil {
				log.Printf("Failed to unmarshal pubsub message data: %v", err)
				return cloudevents.ResultACK
			}
			var ok bool
			if channelID, ok = msgData.Message.Attributes["ce-id"]; !ok {
				log.Print("Failed to read CloudEvent ID from pubsub message")
				return cloudevents.ResultACK
			}
		case schemasv1.CloudStorageObjectFinalizedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				log.Printf("Failed to extract event ID from object name: %v", err)
				return cloudevents.ResultACK
			}
			channelID = channelID + "-" + CloudStorageSourceProbeCreateSubject
		case schemasv1.CloudStorageObjectMetadataUpdatedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				log.Printf("Failed to extract event ID from object name: %v", err)
				return cloudevents.ResultACK
			}
			channelID = channelID + "-" + CloudStorageSourceProbeUpdateMetadataSubject
		case schemasv1.CloudStorageObjectArchivedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				log.Printf("Failed to extract event ID from object name: %v", err)
				return cloudevents.ResultACK
			}
			channelID = channelID + "-" + CloudStorageSourceProbeArchiveSubject
		case schemasv1.CloudStorageObjectDeletedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				log.Printf("Failed to extract event ID from object name: %v", err)
				return cloudevents.ResultACK
			}
			channelID = channelID + "-" + CloudStorageSourceProbeDeleteSubject
		case schemasv1.CloudSchedulerJobExecutedEventType:
			channelID = cloudSchedulerSourceProbeChannelID
		default:
			log.Printf("Unrecognized event type: %v", event.Type())
			return cloudevents.ResultACK
		}

		receivedEvents.RLock()
		defer receivedEvents.RUnlock()
		receiver, ok := receivedEvents.channels[channelID]
		if !ok {
			log.Printf("This event is not received by the probe receiver client: %v \n", channelID)
			return cloudevents.ResultACK
		}
		receiver <- true
		return cloudevents.ResultACK
	}
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

	// create cloud storage client
	csc, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal("Failed to create cloud storage client, ", err)
	}
	bkt := csc.Bucket(env.CloudStorageSourceBucketID)

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

	// make a map to store the channel for each event
	receivedEvents := &receivedEventsMap{
		channels: make(map[string]chan bool),
	}
	// start a goroutine to receive the event from probe and forward it appropriately
	log.Println("Starting Probe Helper server...")
	go sc.StartReceiver(ctx, forwardFromProbe(ctx, sc, psc, bkt, receivedEvents, timeout))
	// Receive the event and return the result back to the probe
	log.Println("Starting event receiver...")
	rc.StartReceiver(ctx, receiveEvent(receivedEvents))
}
