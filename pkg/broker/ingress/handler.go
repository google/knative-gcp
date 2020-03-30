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

package ingress

import (
	"context"
	"fmt"

	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
)

// DecoupleSink is an interface to send events to a decoupling sink (e.g., pubsub).
type DecoupleSink interface {
	// Send sends the event to a decoupling sink.
	Send(ctx context.Context, event cev2.Event) protocol.Result
}

// handler receives events and persists them to storage (pubsub).
type handler struct {
	// inbound is the cloudevents client to receive events.
	inbound cev2.Client
	// decouple is the client to send events to a decouple sink.
	decouple DecoupleSink
}

func NewHandler(ctx context.Context, options ...Option) (*handler, error) {
	h := &handler{}
	for _, option := range options {
		option(h)
	}

	var err error
	if h.inbound == nil {
		if h.inbound, err = newDefaultHTTPClient(); err != nil {
			return nil, fmt.Errorf("failed to create inbound cloudevent client: %w", err)
		}
	}

	if h.decouple == nil {
		if h.decouple, err = h.newDefaultPubSubClient(ctx); err != nil {
			return nil, fmt.Errorf("failed to create decouple cloudevent client: %w", err)
		}
	}

	return h, nil
}

func (h *handler) Start(ctx context.Context) error {
	return h.inbound.StartReceiver(ctx, h.receive)
}

// receive receives events from inbound and sends them to decouple.
func (h *handler) receive(ctx context.Context, event cev2.Event) protocol.Result {
	// TODO(liu-cong) add metrics
	// TODO(liu-cong) route events based on broker.

	if res := h.decouple.Send(ctx, event); !cev2.IsACK(res) {
		logging.FromContext(ctx).Error("Error publishing to PubSub", zap.String("event", event.String()), zap.Error(res))
		return res
	}

	return nil
}

// newDefaultHTTPClient provides good defaults for an HTTP client with tracing.
func newDefaultHTTPClient() (cev2.Client, error) {
	p, err := cev2.NewHTTP()
	if err != nil {
		return nil, err
	}

	return cev2.NewClientObserved(p,
		cev2.WithUUIDs(),
		cev2.WithTimeNow(),
		cev2.WithTracePropagation,
	)
}

// newDefaultPubSubClient creates a pubsub client using env vars "GOOGLE_CLOUD_PROJECT"
// and "PUBSUB_TOPIC"
func (h *handler) newDefaultPubSubClient(ctx context.Context) (cev2.Client, error) {
	// Make a pubsub protocol for the CloudEvents client.
	p, err := pubsub.New(ctx,
		pubsub.WithProjectIDFromDefaultEnv(),
		pubsub.WithTopicIDFromDefaultEnv())
	if err != nil {
		return nil, err
	}

	// Use the pubsub prototol to make a new CloudEvents client.
	return cev2.NewClientObserved(p,
		cev2.WithUUIDs(),
		cev2.WithTimeNow(),
		cev2.WithTracePropagation,
	)
}
