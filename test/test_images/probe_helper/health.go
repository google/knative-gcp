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
	nethttp "net/http"
	"time"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

// healthChecker carries timestamps of the latest handled events from the
// forwarder and receiver, as well as the longest tolerated staleness time.
// The healthChecker is expected to declare the probe helper as unhealthy if
// the probe helper is unable to handle either sort of event.
type healthChecker struct {
	lastProbeEventTimestamp    eventTimestamp
	lastReceiverEventTimestamp eventTimestamp
	maxStaleDuration           time.Duration
}

// stalenessHandlerFunc returns the HTTP handler for probe helper health checks.
func (c *healthChecker) stalenessHandlerFunc(ctx context.Context) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, req *nethttp.Request) {
		if req.URL.Path != "/healthz" {
			logging.FromContext(ctx).Warnw("Invalid health check request path", zap.String("path", req.URL.Path))
			w.WriteHeader(nethttp.StatusNotFound)
			return
		}
		now := time.Now()
		if delay := now.Sub(c.lastProbeEventTimestamp.getTime()); delay > c.maxStaleDuration {
			logging.FromContext(ctx).Warnw("Health check failed, probe delay exceeds staleness threshold", zap.Duration("delay", delay), zap.Duration("staleness", c.maxStaleDuration))
			w.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		if delay := now.Sub(c.lastReceiverEventTimestamp.getTime()); delay > c.maxStaleDuration {
			logging.FromContext(ctx).Warnw("Health check failed, receiver delay exceeds staleness threshold", zap.Duration("delay", delay), zap.Duration("staleness", c.maxStaleDuration))
			w.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		logging.FromContext(ctx).Info("Health check succeeded")
		w.WriteHeader(nethttp.StatusOK)
	}
}
