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

// CloudSchedulerSourceForwardProbe is the probe handler for forward probe requests
// in the CloudSchedulerSource probe.
type CloudSchedulerSourceForwardProbe struct {
	Handler
	channelID string
}

// CloudSchedulerSourceForwardProbeConstructor builds a new CloudSchedulerSource forward
// probe handler from a given CloudEvent.
func CloudSchedulerSourceForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	probe := &CloudSchedulerSourceForwardProbe{
		channelID: fmt.Sprintf("%s/cloudschedulersource-probe", requestHost),
	}
	return probe, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudSchedulerSourceForwardProbe) ChannelID() string {
	return p.channelID
}

// Handle for the CloudSchedulerSourceForwardProbe does nothing.
func (p CloudSchedulerSourceForwardProbe) Handle(ctx context.Context) error {
	return nil
}

// CloudSchedulerSourceReceiveProbe is the probe handler for receiver probe requests
// in the CloudSchedulerSource probe.
type CloudSchedulerSourceReceiveProbe struct {
	Handler
	channelID string
}

// CloudSchedulerSourceReceiveProbeConstructor builds a new CloudSchedulerSource receiver
// probe handler from a given CloudEvent.
func CloudSchedulerSourceReceiveProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
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
	return &CloudAuditLogsSourceForwardProbe{
		channelID: fmt.Sprintf("%s/cloudschedulersource-probe", requestHost),
	}, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p CloudSchedulerSourceReceiveProbe) ChannelID() string {
	return p.channelID
}
