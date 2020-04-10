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
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
)

// Handler pulls Pubsub messages as events and processes them
// with chain of processors.
type Handler struct {
	// PubsubEvents is the CloudEvents Pubsub protocol to pull
	// messages as events.
	PubsubEvents *pubsub.Protocol

	// Processor is the processor to process events.
	Processor processors.Interface

	// Timeout is the timeout for processing each individual event.
	Timeout time.Duration

	// Concurrency is the number of goroutines that will
	// concurrently process events. If not positive, will fall back
	// to 1.
	Concurrency int

	configMut sync.RWMutex
	config    *config.Broker

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc
}

// Start starts the handler.
// done func will be called if the pubsub inbound is closed.
func (h *Handler) Start(ctx context.Context, c *config.Broker, done func(error)) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.config = c
	go func() {
		defer h.cancel()
		done(h.PubsubEvents.OpenInbound(ctx))
	}()

	curr := h.Concurrency
	if curr <= 0 {
		curr = 1
	}
	for i := 0; i < curr; i++ {
		go h.handle(ctx)
	}
}

func (h *Handler) UpdateConfig(c *config.Broker) {
	h.configMut.Lock()
	defer h.configMut.Unlock()
	h.config = c
}

func (h *Handler) getConfig() *config.Broker {
	h.configMut.RLock()
	defer h.configMut.RUnlock()
	return h.config
}

// Stop stops the handlers.
func (h *Handler) Stop() {
	h.cancel()
}

func (h *Handler) handle(ctx context.Context) {
	for {
		// TODO(yolocs): we need to update dep for fix:
		// https://github.com/cloudevents/sdk-go/pull/430
		msg, err := h.PubsubEvents.Receive(ctx)
		if err != nil {
			logging.FromContext(ctx).Error("failed to receive the next message from Pubsub", zap.Error(err))
			continue
		}

		event, err := binding.ToEvent(ctx, msg)
		if err != nil {
			logging.FromContext(ctx).Error("failed to convert received message to an event", zap.Any("message", msg), zap.Error(err))
			continue
		}

		ctx = handlerctx.WithBroker(ctx, h.getConfig())

		pctx := ctx
		if h.Timeout != 0 {
			var cancel context.CancelFunc
			pctx, cancel = context.WithTimeout(ctx, h.Timeout)
			defer cancel()
		}
		if err := h.Processor.Process(pctx, event); err != nil {
			logging.FromContext(pctx).Error("failed to process event", zap.Any("event", event), zap.Error(err))
		}
		// This will ack/nack the message.
		if err := msg.Finish(err); err != nil {
			logging.FromContext(ctx).Warn("failed to finish the message", zap.Any("message", msg), zap.Error(err))
		}
	}
}
