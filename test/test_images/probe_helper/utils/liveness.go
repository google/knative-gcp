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

package utils

import (
	"context"
	nethttp "net/http"
	"time"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

// LivenessChecker carries timestamps of the latest handled events from the
// forwarder and receiver, as well as the longest tolerated staleness time.
// The LivenessChecker fails checks if the probe helper is unable to handle
// either sort of event.
type LivenessChecker struct {
	// LastForwardEventTime is the timestamp of the last event processed by the forward client.
	LastForwardEventTime SyncTime

	// LastReceiverEventTime is the timestamp of the last event processed by the receiver client.
	LastReceiverEventTime SyncTime

	// LivenessStaleDuration represents the tolerated staleness of events processed by the forward and receiver clients in the liveness probe.
	LivenessStaleDuration time.Duration

	// SchedulerStaleDuration represents the tolerated staleness of ticks observed by the CloudSchedulerSource probe.
	SchedulerStaleDuration time.Duration

	// SchedulerEventTimes is the map of latest ticks observed by the CloudSchedulerSource probe.
	SchedulerEventTimes *SyncTimesMap
}

func (c *LivenessChecker) cleanupStaleSchedulerTimes(ctx context.Context) {
	c.SchedulerEventTimes.Lock()
	defer c.SchedulerEventTimes.Unlock()

	for timestampID, schedulerTime := range c.SchedulerEventTimes.Times {
		if delay := time.Now().Sub(schedulerTime); delay.Nanoseconds() > c.SchedulerStaleDuration.Nanoseconds() {
			logging.FromContext(ctx).Infow("Deleting stale scheduler time", zap.String("timestampID", timestampID), zap.Duration("delay", delay))
			delete(c.SchedulerEventTimes.Times, timestampID)
		}
	}
}

// LivenessHandlerFunc returns the HTTP handler for probe helper liveness checks.
func (c *LivenessChecker) LivenessHandlerFunc(ctx context.Context) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, req *nethttp.Request) {
		// Only accept liveness requests along the 'healthz' path
		if req.URL.Path != "/healthz" {
			logging.FromContext(ctx).Warnw("Invalid liveness check request path", zap.String("path", req.URL.Path))
			w.WriteHeader(nethttp.StatusNotFound)
			return
		}

		// Perform cleanup of the stale times in the Cloud Scheduler probe
		c.cleanupStaleSchedulerTimes(ctx)

		// If either of the forward or receiver clients are not processing events, something is wrong
		now := time.Now()
		if delay := now.Sub(c.LastForwardEventTime.Get()); delay > c.LivenessStaleDuration {
			logging.FromContext(ctx).Warnw("Liveness check failed, forward probe delay exceeds staleness threshold", zap.Duration("delay", delay), zap.Duration("staleness", c.LivenessStaleDuration))
			w.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		if delay := now.Sub(c.LastReceiverEventTime.Get()); delay > c.LivenessStaleDuration {
			logging.FromContext(ctx).Warnw("Liveness check failed, receiver delay exceeds staleness threshold", zap.Duration("delay", delay), zap.Duration("staleness", c.LivenessStaleDuration))
			w.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		logging.FromContext(ctx).Info("Liveness check succeeded")
		w.WriteHeader(nethttp.StatusOK)
	}
}
