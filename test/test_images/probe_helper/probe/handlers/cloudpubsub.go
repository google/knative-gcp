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

package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

const (
	// CloudPubSubSourceProbeEventType is the CloudEvent type of forward
	// CloudPubSubSource probes.
	CloudPubSubSourceProbeEventType = "cloudpubsubsource-probe"

	topicExtension = "topic"
)

func NewCloudPubSubSourceProbe(cePubsubClient CePubSubClient) *CloudPubSubSourceProbe {
	return &CloudPubSubSourceProbe{
		cePubsubClient: cePubsubClient,
		receivedEvents: utils.NewSyncReceivedEvents(),
	}
}

type CePubSubClient cloudevents.Client

// CloudPubSubSourceProbe is the probe handler for probe requests in the
// CloudPubSubSource probe.
type CloudPubSubSourceProbe struct {
	// The CloudEvents client responsible for forwarding events as messages to a topic.
	cePubsubClient CePubSubClient

	// The map of received events to be tracked by the forwarder and receiver
	receivedEvents *utils.SyncReceivedEvents
}

// Forward publishes to Pub/Sub in order to generate a notification event.
func (p *CloudPubSubSourceProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe publishes the event as a message to a given Pub/Sub topic.
	topic, ok := event.Extensions()[topicExtension]
	if !ok {
		return fmt.Errorf("CloudPubSubSource probe event has no '%s' extension", topicExtension)
	}
	ctx = cecontext.WithTopic(ctx, fmt.Sprint(topic))
	logging.FromContext(ctx).Infow("Publishing message to pubsub topic", zap.String("topic", fmt.Sprint(topic)))
	if res := p.cePubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
		return fmt.Errorf("Failed sending event to topic %s, got result %s", topic, res)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Receive closes the receiver channel associated with a Pub/Sub notification event.
func (p *CloudPubSubSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// The original event is wrapped into a pubsub Message by the CloudEvents
	// pubsub sender client, and encoded as data in a CloudEvent by the CloudPubSubSource.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: google.cloud.pubsub.topic.v1.messagePublished
	//     source: //pubsub.googleapis.com/projects/project-id/topics/cloudpubsubsource-topic
	//     id: 1529309436535525
	//     time: 2020-09-14T17:06:46.363Z
	//     datacontenttype: application/json
	//   Data,
	//     {
	//       "subscription": "cre-src_events-system-probe_cloudpubsubsource_02f88763-1df6-4944-883f-010ebac27dd2",
	//       "message": {
	//         "messageId": "1529309436535525",
	//         "data": "eydtc2cnOidQcm9iZSBDbG91ZCBSdW4gRXZlbnRzISd9",
	//         "attributes": {
	//           "Content-Type": "application/json",
	//           "ce-id": "cloudpubsubsource-probe-294119a9-98e2-44ec-a2b2-28a98cf40eee",
	//           "ce-source": "probe",
	//           "ce-specversion": "1.0",
	//           "ce-type": "cloudpubsubsource-probe"
	//         },
	//         "publishTime": "2020-09-14T17:06:46.363Z"
	//       }
	//     }
	msgData := schemasv1.PushMessage{}
	if err := json.Unmarshal(event.Data(), &msgData); err != nil {
		return fmt.Errorf("Error unmarshalling Pub/Sub message from event data: %v", err)
	}
	eventID, ok := msgData.Message.Attributes["ce-id"]
	if !ok {
		return fmt.Errorf("Failed to read probe event ID from Pub/Sub message attributes")
	}
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension]), eventID)
	if err := p.receivedEvents.SignalReceiverChannel(channelID); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("Successfully received CloudPubSubSource probe event")
	return nil
}
