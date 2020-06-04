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
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go/v2/binding"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/metrics"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/eventing/pkg/logging"
)

// Handler pulls Pubsub messages as events and processes them
// with chain of processors.
type Handler struct {
	// PubsubEvents is the CloudEvents Pubsub protocol to pull
	// messages as events.
	Subscription *pubsub.Subscription

	// Processor is the processor to process events.
	Processor processors.Interface

	// Timeout is the timeout for processing each individual event.
	Timeout time.Duration

	// RetryLimiter limits how fast to retry failed events.
	RetryLimiter workqueue.RateLimiter

	DelayNack func(time.Duration)

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc
	alive  atomic.Value
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

func (h *Handler) receive(ctx context.Context, msg *pubsub.Message) {
	ctx = metrics.StartEventProcessing(ctx)
	event, err := binding.ToEvent(ctx, cepubsub.NewMessage(msg))
	if err != nil {
		logging.FromContext(ctx).Error("failed to convert received message to an event", zap.Any("message", msg), zap.Error(err))
		msg.Nack()
		return
	}

	if h.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}
	if err := h.Processor.Process(ctx, event); err != nil {
		backoffPeriod := h.RetryLimiter.When(msg.ID)
		logging.FromContext(ctx).Error("failed to process event; backoff nack", zap.Any("event", event), zap.Float64("backoffPeriod", backoffPeriod.Seconds()), zap.Error(err))
		h.DelayNack(backoffPeriod)
		msg.Nack()
		return
	}

	h.RetryLimiter.Forget(msg.ID)
	msg.Ack()
}
