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
	// PingSourceProbeEventType is the CloudEvent type of forward
	// PingSource probes.
	PingSourceProbeEventType = "pingsource-probe"

	pingSourcePeriodExtension = "period"
)

func NewPingSourceProbe(staleDuration time.Duration) *PingSourceProbe {
	return &PingSourceProbe{
		EventTimes: utils.SyncTimesMap{
			Times: map[string]time.Time{},
		},
		StaleDuration: staleDuration,
	}
}

// PingSourceProbe is the probe handler for probe requests in the
// PingSource probe.
type PingSourceProbe struct {
	// The map of times of observed ticks in the PingSource probe
	EventTimes utils.SyncTimesMap

	// StaleDuration is the duration after which entries in the EventTimes map are
	// considered stale and should be cleaned up in the liveness probe.
	StaleDuration time.Duration
}

// Forward tests the delay between the current time and the latest recorded
// PingSource tick in a given scope.
func (p *PingSourceProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	period, ok := event.Extensions()[pingSourcePeriodExtension]
	if !ok {
		return fmt.Errorf("PingSourceProbe event has no '%s' extension", pingSourcePeriodExtension)
	}
	periodDuration, err := time.ParseDuration(fmt.Sprint(period))
	if err != nil {
		return fmt.Errorf("failed to parse PingSource probe period and time.Duration: " + fmt.Sprint(period))
	}

	p.EventTimes.RLock()
	defer p.EventTimes.RUnlock()

	logging.FromContext(ctx).Info("Checking last observed PingSource tick")
	timestampID := fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension])
	pingSourceTime, ok := p.EventTimes.Times[timestampID]
	if !ok {
		return fmt.Errorf("no PingSource tick observed")
	}
	if delay := time.Now().Sub(pingSourceTime); delay.Nanoseconds() > periodDuration.Nanoseconds() {
		return fmt.Errorf("PingSource probe delay %s exceeds period %s", delay, periodDuration)
	}
	return nil
}

// Receive refreshes the latest timestamp for a PingSource tick in a given scope.
func (p *PingSourceProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// Recover the waiting receiver channel ID for the forward PingSource probe.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: dev.knative.sources.ping
	//     source: /apis/v1/namespaces/default/pingsources/test-ping-source-9af24c86-8ba9-4688-80d0-e527678a6a63
	//     id: 1533039115503825
	//     time: 2020-09-15T20:12:00.14Z
	//     datacontenttype: application/json
	//   Data,
	//     { ... }
	p.EventTimes.Lock()
	defer p.EventTimes.Unlock()

	timestampID := fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension])
	p.EventTimes.Times[timestampID] = time.Now()
	logging.FromContext(ctx).Info("Successfully received PingSource probe event")
	return nil
}

// CleanupStalePingSourceTimes returns a handler which loops through each PingSource
// event time and clears the stale entries from the EventTimes map.
func (p *PingSourceProbe) CleanupStalePingSourceTimes() utils.ActionFunc {
	return func(ctx context.Context) error {
		p.EventTimes.Lock()
		defer p.EventTimes.Unlock()

		for timestampID, pingSourceTime := range p.EventTimes.Times {
			if delay := time.Now().Sub(pingSourceTime); delay.Nanoseconds() > p.StaleDuration.Nanoseconds() {
				logging.FromContext(ctx).Infow("Deleting stale PingSource time", zap.String("timestampID", timestampID), zap.Duration("delay", delay))
				delete(p.EventTimes.Times, timestampID)
			}
		}
		return nil
	}
}
