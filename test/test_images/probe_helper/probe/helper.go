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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/test/test_images/probe_helper/probe/handlers"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
)

// withProbeTimeout returns a context with a timeout specified from the 'timeout'
// extension of a given CloudEvent, defaulting to a certain value if not specified,
// and capped to a maximum.
func (ph *Helper) withProbeTimeout(ctx context.Context, event cloudevents.Event) (context.Context, context.CancelFunc) {
	timeout := ph.env.DefaultTimeoutDuration
	if _, ok := event.Extensions()[utils.ProbeEventTimeoutExtension]; ok {
		customTimeoutExtension := fmt.Sprint(event.Extensions()[utils.ProbeEventTimeoutExtension])
		if customTimeout, err := time.ParseDuration(customTimeoutExtension); err != nil {
			logging.FromContext(ctx).Warnw("Failed to parse custom timeout extension duration", zap.String("timeout", customTimeoutExtension), zap.Error(err))
		} else {
			timeout = customTimeout
		}
	}
	if timeout.Nanoseconds() > ph.env.MaxTimeoutDuration.Nanoseconds() {
		logging.FromContext(ctx).Warnw("Desired timeout exceeds the maximum, clamping to maximum value", zap.Duration("timeout", timeout), zap.Duration("maximumTimeout", ph.env.MaxTimeoutDuration))
		timeout = ph.env.MaxTimeoutDuration
	}
	return context.WithTimeout(ctx, timeout)
}

// withProbeEventLoggingContext attaches a logger to the context which contains
// useful information about probe requests.
func withProbeEventLoggingContext(ctx context.Context, event cloudevents.Event) context.Context {
	logger := logging.FromContext(ctx)
	logger = logger.With(zap.Any("event", map[string]interface{}{
		"id":          event.ID(),
		"source":      event.Source(),
		"specversion": event.SpecVersion(),
		"type":        event.Type(),
		"extensions":  event.Extensions(),
	}))
	return logging.WithLogger(ctx, logger)
}

type cloudEventsFunc func(cloudevents.Event) cloudevents.Result

// forwardFromProbe is the base forward probe request handler which is called
// whenever the probe helper receives a CloudEvent through port PROBE_PORT or
// through the specified probe port listener.
func (ph *Helper) forwardFromProbe(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) cloudevents.Result {
		// Attach important metadata about the event to the logging context.
		ctx := withProbeEventLoggingContext(ctx, event)
		// Scope this to debug level log to avoid log clutter in case of unintended probe requests.
		logging.FromContext(ctx).Debugw("Received probe request")

		// Refresh the forward probe liveness time
		ph.lastForwardEventTime.SetNow()

		// Ensure there is a targetpath CloudEvent extension
		if _, ok := event.Extensions()[utils.ProbeEventTargetPathExtension]; !ok {
			logging.FromContext(ctx).Debugf("Probe forwarding failed, forward probe event missing '%s' extension", utils.ProbeEventTargetPathExtension)
			return cloudevents.ResultNACK
		}

		// Add timeout to the context
		ctx, cancel := ph.withProbeTimeout(ctx, event)
		defer cancel()

		// Forward the probe event. This call is likely to be blocking.
		if err := ph.probeHandler.Forward(ctx, event); err != nil {
			logging.FromContext(ctx).Debugw("Probe forwarding failed", zap.Error(err))
			return cloudevents.ResultNACK
		}
		return cloudevents.ResultACK
	}
}

// receiveEvent is the base receiver probe request handler which is called
// whenever the probe helper receives a CloudEvent through port RECEIVER_PORT or
// through the specified receiver port listener.
func (ph *Helper) receiveEvent(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) cloudevents.Result {
		// Attach important metadata about the event to the logging context.
		ctx := withProbeEventLoggingContext(ctx, event)
		// Scope this to debug level log to avoid log clutter in case of unintended probe requests.
		logging.FromContext(ctx).Debugw("Received event")

		// Refresh the receiver probe liveness time
		ph.lastReceiverEventTime.SetNow()

		// Ensure there is a receiverpath CloudEvent extension
		if _, ok := event.Extensions()[utils.ProbeEventReceiverPathExtension]; !ok {
			logging.FromContext(ctx).Debugf("Probe receiver failed, receiver probe event missing '%s' extension", utils.ProbeEventReceiverPathExtension)
			return cloudevents.ResultACK
		}

		// Receive the probe event
		if err := ph.probeHandler.Receive(ctx, event); err != nil {
			logging.FromContext(ctx).Debugw("Probe receiver failed", zap.Error(err))
			return cloudevents.ResultACK
		}
		return cloudevents.ResultACK
	}
}

// CheckLastEventTimes returns an actionFunc which checks the delay between the
// current time and last processed event times from the forward and receiver
// clients. This handler is used by the liveness checker to declare the liveness
// status of the probe helper.
func (ph *Helper) CheckLastEventTimes() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// If either of the forward or receiver clients are not processing events, something is wrong
		now := time.Now()
		if delay := now.Sub(ph.lastForwardEventTime.Get()); delay > ph.env.LivenessStaleDuration {
			return fmt.Errorf("forward delay %s exceeds staleness threshold %s", delay, ph.env.LivenessStaleDuration)
		}
		if delay := now.Sub(ph.lastReceiverEventTime.Get()); delay > ph.env.LivenessStaleDuration {
			return fmt.Errorf("receiver delay %s exceeds staleness threshold %s", delay, ph.env.LivenessStaleDuration)
		}
		return nil
	}
}

// Run starts the probe forwarder and receiver. This function should be called
// after Initialize.
func (ph *Helper) Run(ctx context.Context) {
	// Start a goroutine to receive the probe request event and forward it appropriately
	logging.FromContext(ctx).Infow("Starting event forwarder client...")
	go ph.ceForwardClient.StartReceiver(ctx, ph.forwardFromProbe(ctx))

	// Receive the event and return the result back to the probe
	logging.FromContext(ctx).Infow("Starting event receiver client...")
	ph.ceReceiveClient.StartReceiver(ctx, ph.receiveEvent(ctx))
}

// Helper is the main probe helper object which contains the metadata and clients
// shared between all probe Handlers.
type Helper struct {
	env EnvConfig

	// The client responsible for handling forward probe requests
	ceForwardClient handlers.CeForwardClient
	// The client responsible for receiving events
	ceReceiveClient handlers.CeReceiveClient

	// The liveness checker invoked in the liveness probe
	livenessChecker *utils.LivenessChecker

	probeHandler handlers.Interface

	// lastForwardEventTime is the timestamp of the last event processed by the forward client.
	lastForwardEventTime utils.SyncTime

	// lastReceiverEventTime is the timestamp of the last event processed by the receiver client.
	lastReceiverEventTime utils.SyncTime
}

type EnvConfig struct {
	// Environment variable containing the maximum tolerated staleness duration for events processed by the forward and receiver clients
	LivenessStaleDuration time.Duration `envconfig:"LIVENESS_STALE_DURATION" default:"5m"`

	// Environment variable containing the default timeout duration to wait for an event to be delivered, if no custom timeout is specified
	DefaultTimeoutDuration time.Duration `envconfig:"DEFAULT_TIMEOUT_DURATION" default:"2m"`

	// Environment variable containing the maximum timeout duration to wait for an event to be delivered
	MaxTimeoutDuration time.Duration `envconfig:"MAX_TIMEOUT_DURATION" default:"30m"`
}
