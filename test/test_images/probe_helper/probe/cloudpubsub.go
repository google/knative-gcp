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

package probe

import (
	"context"
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
	// CloudPubSubSourceProbeEventType is the CloudEvent type of forward
	// CloudPubSubSource probes.
	CloudPubSubSourceProbeEventType = "cloudpubsubsource-probe"

	topicExtension = "topic"
)

// CloudPubSubSourceForwardProbe is the probe handler for forward probe requests
// in the CloudPubSubSource probe.
type CloudPubSubSourceForwardProbe struct {
	Handler
	event          cloudevents.Event
	channelID      string
	topic          string
	cePubsubClient cloudevents.Client
}

// CloudPubSubSourceForwardProbeConstructor builds a new CloudPubSubSource forward
// probe handler from a given CloudEvent.
func CloudPubSubSourceForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	topic, ok := event.Extensions()[topicExtension]
	if !ok {
		return nil, fmt.Errorf("CloudPubSubSource probe event has no '%s' extension", topicExtension)
	}
	probe := &CloudPubSubSourceForwardProbe{
		event:          event,
		channelID:      fmt.Sprintf("%s/%s", requestHost, event.ID()),
		topic:          fmt.Sprint(topic),
		cePubsubClient: ph.CePubsubClient,
	}
	return probe, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudPubSubSourceForwardProbe) ChannelID() string {
	return p.channelID
}

// Handle forwards a given CloudEvent as a message to a Pub/Sub topic.
func (p CloudPubSubSourceForwardProbe) Handle(ctx context.Context) error {
	ctx = cecontext.WithTopic(ctx, p.topic)
	if res := p.cePubsubClient.Send(ctx, p.event); !cloudevents.IsACK(res) {
		return fmt.Errorf("Failed sending event to topic %s, got result %s", p.topic, res)
	}
	return nil
}

// CloudPubSubSourceReceiveProbe is the probe handler for receiver probe requests
// in the CloudPubSubSource probe.
type CloudPubSubSourceReceiveProbe struct {
	Handler
	channelID string
}

// CloudPubSubSourceReceiveProbeConstructor builds a new CloudPubSubSource receiver
// probe handler from a given CloudEvent.
func CloudPubSubSourceReceiveProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
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
	//       "subscription": "cre-src_cloud-run-events-probe_cloudpubsubsource_02f88763-1df6-4944-883f-010ebac27dd2",
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
		return nil, fmt.Errorf("Error unmarshalling Pub/Sub message from event data: %v", err)
	}
	eventID, ok := msgData.Message.Attributes["ce-id"]
	if !ok {
		return nil, fmt.Errorf("Failed to read probe event ID from Pub/Sub message attributes")
	}
	return &CloudPubSubSourceReceiveProbe{
		channelID: fmt.Sprintf("%s/%s", requestHost, eventID),
	}, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudPubSubSourceReceiveProbe) ChannelID() string {
	return p.channelID
}
