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

	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

// DecoupleSink is an interface to send events to a decoupling sink (e.g., pubsub).
type DecoupleSink interface {
	// Send sends the event to a decoupling sink.
	Send(ctx context.Context, event event.Event) error
}

// handler receives events and persists them to storage (pubsub).
type handler struct {
	// inbound is the cloudevents client to receive events.
	inbound client.Client
	// decouple is the client to send events to a decouple sink.
	decouple DecoupleSink
}

func NewHandler(ctx context.Context, options ...Option) (*handler, error) {
	h := &handler{}
	var err error
	// Create a default HTTP client for ingress.
	if h.inbound, err = client.NewDefault(); err != nil {
		return nil, fmt.Errorf("failed to create inbound cloudevent client: %w", err)
	}
	// Create a pubsub client to send events to decoupling topic.
	if h.decouple, err = h.newDefaultPubSubClient(ctx); err != nil {
		return nil, fmt.Errorf("failed to create decouple cloudevent client: %w", err)
	}

	for _, option := range options {
		option(h)
	}
	return h, nil
}

func (h *handler) Start(ctx context.Context) error {
	return h.inbound.StartReceiver(ctx, h.receive)
}

// receive receives events from inbound and sends them to decouple.
func (h *handler) receive(ctx context.Context, event event.Event) protocol.Result {
	// Code below is for ce v1.
	// TODO(liu-cong) figure out tracing support for ce v2
	// event = tracing.AddTraceparentAttributeFromContext(ctx, event)

	// TODO(liu-cong) add metrics

	// TODO(liu-cong) route events based on broker.

	if err := h.decouple.Send(ctx, event); err != nil {
		logging.FromContext(ctx).Desugar().Error("Error publishing to PubSub", zap.String("event", event.String()), zap.Error(err))
		return err
	}

	return nil
}

// newDefaultPubSubClient creates a pubsub client using env vars "GOOGLE_CLOUD_PROJECT"
// and "PUBSUB_TOPIC"
func (h *handler) newDefaultPubSubClient(ctx context.Context) (client.Client, error) {
	// Make a pubsub protocol for the CloudEvents client.
	p, err := pubsub.New(ctx,
		pubsub.WithProjectIDFromDefaultEnv(),
		pubsub.WithTopicIDFromDefaultEnv())
	if err != nil {
		return nil, err
	}

	// Use the pubsub prototol to make a new CloudEvents client.
	return client.NewObserved(p,
		client.WithUUIDs(),
		client.WithTimeNow(),
	)
}
