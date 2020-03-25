/*
Copyright 2019 Google LLC

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

package publisher

import (
	"fmt"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"context"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	cepubsub "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub"
	"knative.dev/eventing/pkg/kncloudevents"
)

// Publisher implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Publisher struct {
	// ProjectID is the pre-existing eventing project id to use.
	ProjectID string
	// TopicID is the pre-existing eventing pub/sub topic id to use.
	TopicID string

	// inbound is the cloudevents client to use to receive events.
	inbound cloudevents.Client
	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client
}

func (a *Publisher) Start(ctx context.Context) error {
	var err error

	// Receive events on HTTP.
	if a.inbound == nil {
		if a.inbound, err = kncloudevents.NewDefaultClient(); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %w", err)
		}
	}

	// Send Events on Pub/Sub.
	if a.outbound == nil {
		if a.outbound, err = a.newPubSubClient(ctx); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %w", err)
		}
	}

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Publisher) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	if _, r, err := a.outbound.Send(ctx, event); err != nil {
		logging.FromContext(ctx).Desugar().Error("Error publishing to PubSub", zap.String("event", event.String()), zap.Error(err))
		return err
	} else if r != nil {
		resp.RespondWith(200, r)
	}

	return nil
}

func (a *Publisher) newPubSubClient(ctx context.Context) (cloudevents.Client, error) {
	tOpts := []cepubsub.Option{
		cepubsub.WithBinaryEncoding(),
		cepubsub.WithProjectID(a.ProjectID),
		cepubsub.WithTopicID(a.TopicID),
	}

	// Make a pubsub transport for the CloudEvents client.
	t, err := cepubsub.New(ctx, tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	return cloudevents.NewClient(t,
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)
}
