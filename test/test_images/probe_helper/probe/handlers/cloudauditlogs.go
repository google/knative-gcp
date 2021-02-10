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
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

const (
	// CloudAuditLogsSourceProbeEventType is the CloudEvent type of forward
	// CloudAuditLogsSource probes.
	CloudAuditLogsSourceProbeEventType = "cloudauditlogssource-probe"
)

func NewCloudAuditLogsSourceProbe(projectID clients.ProjectID, pubsubClient *pubsub.Client) *CloudAuditLogsSourceProbe {
	return &CloudAuditLogsSourceProbe{
		projectID:      projectID,
		pubsubClient:   pubsubClient,
		receivedEvents: utils.NewSyncReceivedEvents(),
	}
}

// CloudAuditLogsSourceProbe is the probe handler for probe requests in the
// CloudAuditLogsSource probe.
type CloudAuditLogsSourceProbe struct {
	// The project ID
	projectID clients.ProjectID

	// The pubsub client wrapped by a CloudEvents client for the CloudPubSubSource
	// probe and used for the CloudAuditLogsSource probe
	pubsubClient *pubsub.Client

	// The map of received events to be tracked by the forwarder and receiver
	receivedEvents *utils.SyncReceivedEvents
}

// Forward creates a Pub/Sub topic in order to generate a Cloud Audit Logs notification event.
func (p *CloudAuditLogsSourceProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe creates a Pub/Sub topic.
	topic := event.ID()
	logging.FromContext(ctx).Infow("Creating pubsub topic", zap.String("topic", topic))
	if _, err := p.pubsubClient.CreateTopic(ctx, topic); err != nil {
		return fmt.Errorf("Failed to create pubsub topic '%s': %v", topic, err)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Receive closes the receiver channel associated with a Cloud Audit Logs notification event.
func (p *CloudAuditLogsSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// The logged event type is held in the methodname extension. For creation
	// of pubsub topics, the topic ID can be extracted from the event subject.
	if _, ok := event.Extensions()["methodname"]; !ok {
		return fmt.Errorf("Failed to read Cloud AuditLogs event, missing 'methodname' extension")
	}
	sepSub := strings.Split(event.Subject(), "/")
	if len(sepSub) != 5 || sepSub[0] != "pubsub.googleapis.com" || sepSub[1] != "projects" || sepSub[2] != string(p.projectID) || sepSub[3] != "topics" {
		return fmt.Errorf("Failed to read Cloud AuditLogs event, unexpected event subject")
	}
	methodname := fmt.Sprint(event.Extensions()["methodname"])
	var eventID string
	if methodname == "google.pubsub.v1.Publisher.CreateTopic" {
		// Example:
		//   Context Attributes,
		//     specversion: 1.0
		//     type: google.cloud.audit.log.v1.written
		//     source: //cloudaudit.googleapis.com/projects/project-id/logs/activity
		//     subject: pubsub.googleapis.com/projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
		//     id: d2ad1359483fc13c8056c430545fd217
		//     time: 2020-09-14T18:44:18.636961725Z
		//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto
		//     datacontenttype: application/json
		//   Extensions,
		//     methodname: google.pubsub.v1.Publisher.CreateTopic
		//     resourcename: projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
		//     servicename: pubsub.googleapis.com
		//   Data,
		//     { ... }
		eventID = sepSub[4]
	} else {
		return fmt.Errorf("Failed to read Cloud AuditLogs event, unrecognized 'methodname' extension: %s", methodname)
	}
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension]), eventID)
	if err := p.receivedEvents.SignalReceiverChannel(channelID); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("Successfully received CloudAuditLogsSource probe event")
	return nil
}
