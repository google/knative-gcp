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
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Probe event goes here
type CloudAuditLogsSourceForwardProbe struct {
	ProbeInterface
	event        cloudevents.Event
	channelID    string
	pubsubClient *pubsub.Client
}

func CloudAuditLogsSourceForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	//requestHost, ok := event.Extensions()[ProbeEventRequestHostExtension]
	//if !ok {
	//	return nil, fmt.Errorf("Failed to read '%s' extension", ProbeEventRequestHostExtension)
	//}
	probe := &CloudAuditLogsSourceForwardProbe{
		event:        event,
		channelID:    event.ID(),
		pubsubClient: ph.pubsubClient,
	}
	return probe, nil
}

func (p CloudAuditLogsSourceForwardProbe) ChannelID() string {
	return p.channelID
}

func (p CloudAuditLogsSourceForwardProbe) Handle(ctx context.Context) error {
	// Create a pubsub topic with the given ID.
	if _, err := p.pubsubClient.CreateTopic(ctx, p.event.ID()); err != nil {
		return fmt.Errorf("Failed to create pubsub topic '%s': %v", p.event.ID(), err)
	}
	return nil
}

// Receiver event goes here
type CloudAuditLogsSourceReceiveProbe struct {
	ProbeInterface
	channelID string
}

func CloudAuditLogsSourceReceiveProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	// The logged event type is held in the methodname extension. For creation
	// of pubsub topics, the topic ID can be extracted from the event subject.
	if _, ok := event.Extensions()["methodname"]; !ok {
		return nil, fmt.Errorf("Failed to read Cloud AuditLogs event, missing 'methodname' extension")
	}
	sepSub := strings.Split(event.Subject(), "/")
	if len(sepSub) != 5 || sepSub[0] != "pubsub.googleapis.com" || sepSub[1] != "projects" || sepSub[2] != ph.projectID || sepSub[3] != "topics" {
		return nil, fmt.Errorf("Failed to read Cloud AuditLogs event, unexpected event subject")
	}
	methodname := fmt.Sprint(event.Extensions()["methodname"])
	var channelID string
	if methodname == "google.pubsub.v1.Publisher.CreateTopic" {
		// Example:
		//   Context Attributes,
		//     specversion: 1.0
		//     type: google.cloud.audit.log.v1.written
		//     source: //cloudaudit.googleapis.com/projects/project-id/logs/activity
		//     subject: pubsub.googleapis.com/projects/project-id/topics/914e5946-5e27-4bde-a455-7cfbae1c8539
		//     id: d2ad1359483fc13c8056c430545fd217
		//     time: 2020-09-14T18:44:18.636961725Z
		//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto
		//     datacontenttype: application/json
		//   Extensions,
		//     methodname: google.pubsub.v1.Publisher.CreateTopic
		//     resourcename: projects/project-id/topics/914e5946-5e27-4bde-a455-7cfbae1c8539
		//     servicename: pubsub.googleapis.com
		//   Data,
		//     { ... }
		channelID = sepSub[4]
	} else {
		return nil, fmt.Errorf("Failed to read Cloud AuditLogs event, unrecognized 'methodname' extension: %s", methodname)
	}
	return &CloudAuditLogsSourceReceiveProbe{
		channelID: channelID,
	}, nil
}

func (p CloudAuditLogsSourceReceiveProbe) ChannelID() string {
	return p.channelID
}
