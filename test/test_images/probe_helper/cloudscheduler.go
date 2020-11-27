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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	periodExtension = "period"
)

// Probe event goes here
type CloudSchedulerSourceForwardProbe struct {
	ProbeInterface
	event                       cloudevents.Event
	period                      time.Duration
	lastSchedulerEventTimestamp *eventTimestamp
}

func CloudSchedulerSourceForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	//requestHost, ok := event.Extensions()[ProbeEventRequestHostExtension]
	//if !ok {
	//	return nil, fmt.Errorf("Failed to read '%s' extension", ProbeEventRequestHostExtension)
	//}
	periodString, ok := event.Extensions()[periodExtension]
	if !ok {
		return nil, fmt.Errorf("CloudSchedulerSource probe event has no '%s' extension", periodExtension)
	}
	period, err := time.ParseDuration(fmt.Sprint(periodString))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse period %s as time.Duration: %v", periodString, err)
	}
	probe := &CloudSchedulerSourceForwardProbe{
		event:                       event,
		period:                      period,
		lastSchedulerEventTimestamp: ph.lastSchedulerEventTimestamp,
	}
	return probe, nil
}

func (p CloudSchedulerSourceForwardProbe) ChannelID() string {
	return ""
}

func (p CloudSchedulerSourceForwardProbe) Handle(ctx context.Context) error {
	// Fail if the delay since the last scheduled tick is greater than the desired period.
	if delay := time.Now().Sub(p.lastSchedulerEventTimestamp.getTime()); delay > p.period {
		return fmt.Errorf("Delay %s between CloudSchedulerSource ticks exceeds period %s", delay, p.period)
	}
	return nil
}

// Receiver event goes here
type CloudSchedulerSourceReceiveProbe struct {
	ProbeInterface
}

func CloudSchedulerSourceReceiveProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	// Refresh the last received event timestamp from the CloudSchedulerSource.
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
	ph.lastSchedulerEventTimestamp.setNow()
	return nil, nil
}

func (p CloudSchedulerSourceReceiveProbe) ChannelID() string {
	return ""
}
