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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

const (
	// CloudSchedulerSourceProbeEventType is the CloudEvent type of forward
	// CloudSchedulerSource probes.
	CloudSchedulerSourceProbeEventType = "cloudschedulersource-probe"

	cloudSchedulerPeriodExtension = "period"
)

func NewCloudSchedulerSourceProbe(staleDuration time.Duration) *CloudSchedulerSourceProbe {
	return &CloudSchedulerSourceProbe{
		EventTimes: utils.SyncTimesMap{
			Times: map[string]time.Time{},
		},
		StaleDuration: staleDuration,
	}
}

// CloudSchedulerSourceProbe is the probe handler for probe requests in the
// CloudSchedulerSource probe.
type CloudSchedulerSourceProbe struct {
	// The map of times of observed ticks in the CloudSchedulerSource probe
	EventTimes utils.SyncTimesMap

	// StaleDuration is the duration after which entries in the EventTimes map are
	// considered stale and should be cleaned up in the liveness probe.
	StaleDuration time.Duration
}

// Forward tests the delay between the current time and the latest recorded Cloud
// Scheduler tick in a given scope.
func (p *CloudSchedulerSourceProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	period, ok := event.Extensions()[cloudSchedulerPeriodExtension]
	if !ok {
		return fmt.Errorf("CloudSchedulerProbe event has no '%s' extension", cloudSchedulerPeriodExtension)
	}
	periodDuration, err := time.ParseDuration(fmt.Sprint(period))
	if err != nil {
		return fmt.Errorf("failed to parse CloudSchedulerSource probe period and time.Duration: " + fmt.Sprint(period))
	}

	p.EventTimes.RLock()
	defer p.EventTimes.RUnlock()

	logging.FromContext(ctx).Infow("Checking last observed scheduler tick")
	timestampID := fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension])
	schedulerTime, ok := p.EventTimes.Times[timestampID]
	if !ok {
		return fmt.Errorf("no scheduler tick observed")
	}
	if delay := time.Now().Sub(schedulerTime); delay.Nanoseconds() > periodDuration.Nanoseconds() {
		return fmt.Errorf("scheduler probe delay %s exceeds period %s", delay, periodDuration)
	}
	return nil
}

// Receive refreshes the latest timestamp for a Cloud Scheduler tick in a given scope.
func (p *CloudSchedulerSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
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
	p.EventTimes.Lock()
	defer p.EventTimes.Unlock()

	timestampID := fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension])
	p.EventTimes.Times[timestampID] = time.Now()
	logging.FromContext(ctx).Info("Successfully received CloudSchedulerSource probe event")
	return nil
}

// CleanupStaleSchedulerTimes returns a handler which loops through each scheduler
// event time and clears the stale entries from the EventTimes map.
func (p *CloudSchedulerSourceProbe) CleanupStaleSchedulerTimes() utils.ActionFunc {
	return func(ctx context.Context) error {
		p.EventTimes.Lock()
		defer p.EventTimes.Unlock()

		for timestampID, schedulerTime := range p.EventTimes.Times {
			if delay := time.Now().Sub(schedulerTime); delay.Nanoseconds() > p.StaleDuration.Nanoseconds() {
				logging.FromContext(ctx).Infow("Deleting stale scheduler time", zap.String("timestampID", timestampID), zap.Duration("delay", delay))
				delete(p.EventTimes.Times, timestampID)
			}
		}
		return nil
	}
}
