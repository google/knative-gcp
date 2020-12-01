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

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// CloudSchedulerSourceProbeEventType is the CloudEvent type of forward
	// CloudSchedulerSource probes.
	CloudSchedulerSourceProbeEventType = "cloudschedulersource-probe"
)

// CloudSchedulerSourceProbe is the probe handler for probe requests in the
// CloudSchedulerSource probe.
type CloudSchedulerSourceProbe struct{}

var cloudSchedulerSourceProbe Handler = &CloudSchedulerSourceProbe{}

func (p *CloudSchedulerSourceProbe) Forward(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Get the receiver channel
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], "cloudschedulersource-probe")
	receiverChannel, cleanupFunc, err := CreateReceiverChannel(ctx, ph, channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	return WaitOnReceiverChannel(ctx, receiverChannel)
}

func (p *CloudSchedulerSourceProbe) Receive(ctx context.Context, ph *Helper, event cloudevents.Event) error {
	// Recover the waiting receiver channel ID for the forward CloudSchedulerSource probe.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: google.cloud.scheduler.job.v1.executed
	//     source: //cloudscheduler.googleapis.com/projects/project-id/locations/location/jobs/cre-scheduler-9af24c86-8ba9-4688-80d0-e527678a6a63
	//     id: 1533039115503825
	//     time: 2020-09-15T20:12:00.14Z
	//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/scheduler/v1/data.proto
	//     datacontenttype: application/json
	//   Data,
	//     { ... }
	channelID := fmt.Sprintf("%s/%s", event.Extensions()[probeEventTargetServiceExtension], "cloudschedulersource-probe")
	return CloseReceiverChannel(ctx, ph, channelID)
}
