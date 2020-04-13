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

	ConfigCh chan *config.Broker

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc

	done <-chan struct{}
}

// Start starts the handler.
// done func will be called if the pubsub inbound is closed.
func (h *Handler) Start(ctx context.Context, config *config.Broker) {
	ctx, h.cancel = context.WithCancel(ctx)
	h.done = ctx.Done()
	go func() {
		defer h.cancel()
		err := h.PubsubEvents.OpenInbound(ctx)
		logging.FromContext(ctx).Error("pubsub client closed", zap.Error(err))
	}()

	go h.handle(ctx, config)
}

// Stop stops the handlers.
func (h *Handler) Stop() {
	h.cancel()
}

// Done returns a channel indicating when the handler has stopped.
func (h *Handler) Done() <-chan struct{} {
	return h.done
}

type pubsubResponse struct {
	msg binding.Message
	err error
}

func (h *Handler) handle(ctx context.Context, config *config.Broker) {
	limit := h.Concurrency
	if limit < 1 {
		limit = 1
	}
	tokens := make(chan struct{}, limit)

	var msgCh chan pubsubResponse

	for {
		var getToken chan struct{}
		if msgCh == nil {
			getToken = tokens
		}
		select {
		case getToken <- struct{}{}:
			msgCh = make(chan pubsubResponse)
			go func(ctx context.Context, msgCh chan<- pubsubResponse) {
				// TODO(yolocs): we need to update dep for fix:
				// https://github.com/cloudevents/sdk-go/pull/430
				msg, err := h.PubsubEvents.Receive(ctx)
				select {
				case msgCh <- pubsubResponse{msg, err}:
				case <-ctx.Done():
					if err == nil {
						if err := msg.Finish(ctx.Err()); err != nil {
							logging.FromContext(ctx).Warn("failed to finish the message", zap.Any("message", msg), zap.Error(err))
						}
					}
				}
			}(ctx, msgCh)
		case r := <-msgCh:
			msgCh = nil
			if r.err != nil {
				logging.FromContext(ctx).Error("failed to receive the next message from Pubsub", zap.Error(r.err))
				<-tokens
			} else {
				go h.handleMessage(handlerctx.WithBroker(ctx, config), r.msg, tokens)
			}
		case config = <-h.ConfigCh:
		case <-ctx.Done():
			return
		}
	}
}

func (h *Handler) handleMessage(ctx context.Context, msg binding.Message, doneCh <-chan struct{}) {
	defer func() { <-doneCh }()

	event, err := binding.ToEvent(ctx, msg)
	if err != nil {
		logging.FromContext(ctx).Error("failed to convert received message to an event", zap.Any("message", msg), zap.Error(err))
		return
	}

	if h.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}
	if err = h.Processor.Process(ctx, event); err != nil {
		logging.FromContext(ctx).Error("failed to process event", zap.Any("event", event), zap.Error(err))
	}
	// This will ack/nack the message.
	if err := msg.Finish(err); err != nil {
		logging.FromContext(ctx).Warn("failed to finish the message", zap.Any("message", msg), zap.Error(err))
	}
}
