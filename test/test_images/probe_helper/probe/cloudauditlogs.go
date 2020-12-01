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
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// CloudAuditLogsSourceProbeEventType is the CloudEvent type of forward
	// CloudAuditLogsSource probes.
	CloudAuditLogsSourceProbeEventType = "cloudauditlogssource-probe"
)

// CloudAuditLogsSourceProbe is the probe handler for probe requests in the
// CloudAuditLogsSource probe.
type CloudAuditLogsSourceProbe struct{}

var cloudAuditLogsSourceProbe Handler = &CloudAuditLogsSourceProbe{}

func (p *CloudAuditLogsSourceProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], event.ID())
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe creates a Pub/Sub topic.
	topic := event.ID()
	if _, err := ph.PubsubClient.CreateTopic(ctx, topic); err != nil {
		return fmt.Errorf("Failed to create pubsub topic '%s': %v", topic, err)
	}

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudAuditLogsSourceProbe) Receive(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// The logged event type is held in the methodname extension. For creation
	// of pubsub topics, the topic ID can be extracted from the event subject.
	if _, ok := event.Extensions()["methodname"]; !ok {
		return fmt.Errorf("Failed to read Cloud AuditLogs event, missing 'methodname' extension")
	}
	sepSub := strings.Split(event.Subject(), "/")
	if len(sepSub) != 5 || sepSub[0] != "pubsub.googleapis.com" || sepSub[1] != "projects" || sepSub[2] != ph.ProjectID || sepSub[3] != "topics" {
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
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], eventID)
	return CloseReceiverChannel(ctx, ph, channelID)
}
