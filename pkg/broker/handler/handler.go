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

package handler

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/logging"
	"github.com/google/knative-gcp/pkg/metrics"
	"go.uber.org/zap"
)

// Handler pulls Pubsub messages as events and processes them
// with chain of processors.
type Handler struct {
	// Subscription is the pubsub subscription that messages will be
	// received from.
	Subscription *pubsub.Subscription

	// Processor is the processor to process events.
	Processor processors.Interface

	// Timeout is the timeout for processing each individual event.
	Timeout time.Duration

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc

	// alive is a bool indicator that the handler is still alive.
	alive atomic.Value
}

// NewHandler creates a new Handler.
func NewHandler(
	sub *pubsub.Subscription,
	processor processors.Interface,
	timeout time.Duration,
) *Handler {
	return &Handler{
		Subscription: sub,
		Processor:    processor,
		Timeout:      timeout,
	}
}

// Start starts the handler.
// done func will be called if the pubsub inbound is closed.
func (h *Handler) Start(ctx context.Context, done func(error)) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.alive.Store(true)

	go func() {
		// For any reason if inbound is closed, mark alive as false.
		defer h.alive.Store(false)
		done(h.Subscription.Receive(ctx, h.receive))
	}()
}

// Stop stops the handlers.
func (h *Handler) Stop() {
	h.cancel()
}

// IsAlive indicates whether the handler is alive.
func (h *Handler) IsAlive() bool {
	return h.alive.Load().(bool)
}

// receive converts message to events and invoke processor chain.
func (h *Handler) receive(ctx context.Context, msg *pubsub.Message) {
	ctx = metrics.StartEventProcessing(ctx)
	event, err := binding.ToEvent(ctx, cepubsub.NewMessage(msg))
	if isNonRetryable(err) {
		logEventConversionError(ctx, msg, err, "failed to convert received message to an event, check the msg format")
		// Ack the message so it won't be retried.
		// TODO Should this go to the DLQ once DLQ is implemented?
		msg.Ack()
		return
	}
	if err != nil {
		logEventConversionError(ctx, msg, err, "unknown error when converting the received message to an event")
		msg.Nack()
		return
	}

	if h.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}
	if err := h.Processor.Process(ctx, event); err != nil {
		logging.FromContext(ctx).Error("failed to process event", zap.String("eventID", event.ID()), zap.Error(err))
		msg.Nack()
		return
	}

	msg.Ack()
}

func isNonRetryable(err error) bool {
	// The following errors can be returned by ToEvent and are not retryable.
	// TODO Should binding.ToEvent consolidate them and return the generic ErrCannotConvertToEvent?
	return errors.Is(err, binding.ErrCannotConvertToEvent) || errors.Is(err, binding.ErrNotStructured) || errors.Is(err, binding.ErrUnknownEncoding) || errors.Is(err, binding.ErrNotBinary)
}

// Log the full message in debug level and a truncated version as an error in case the message is too big (can be as big as 10MB),
func logEventConversionError(ctx context.Context, pm *pubsub.Message, err error, msg string) {
	maxLen := 2000
	truncated := pm
	if len(pm.Data) > maxLen {
		copy := *pm
		copy.Data = copy.Data[:maxLen]
		truncated = &copy
	}
	logging.FromContext(ctx).Debug(msg, zap.Any("message", pm), zap.Error(err))
	logging.FromContext(ctx).Error(msg, zap.Any("message-truncated", truncated), zap.Error(err))
}
