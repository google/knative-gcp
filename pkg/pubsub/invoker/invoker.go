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

package invoker

import (
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"github.com/google/uuid"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
)

// Invoker implements the Pub/Sub invoker to deliver Pub/Sub messages from a
// pre-existing topic with a set of pre-existing subscriptions to the set of
// Channel Subscribers.
type Invoker struct {
	// ProjectID is the pre-existing eventing project id to use.
	ProjectID string

	// inbound is the cloudevents client to use to receive events.
	inbound          cloudevents.Client
	inboundTransport *cepubsub.Transport

	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client
}

func (a *Invoker) Start(ctx context.Context) error {
	var err error

	// Receive Events on Pub/Sub.
	if a.inbound == nil {
		if a.inbound, err = a.newPubSubClient(ctx); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %s", err.Error())
		}
	}

	// Send events on HTTP.
	if a.outbound == nil {
		if a.outbound, err = kncloudevents.NewDefaultClient("TODO"); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %s", err.Error())
		}
	}

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Invoker) DeletePubSubSubscription(ctx context.Context) error {
	if a.inboundTransport != nil {
		return a.inboundTransport.DeleteSubscription(ctx)
	}
	return errors.New("no inbound transport set")
}

func (a *Invoker) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	//logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))

	if r, err := a.outbound.Send(ctx, event); err != nil {
		return err
	} else if r != nil {
		resp.RespondWith(200, r)
	}

	return nil
}

func (a *Invoker) convert(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
	if msg, ok := m.(*cepubsub.Message); ok {
		tx := cepubsub.TransportContextFrom(ctx)
		// Make a new event and convert the message payload.
		event := cloudevents.NewEvent()
		event.SetID(tx.ID)
		event.SetTime(tx.PublishTime)
		event.SetSource(v1alpha1.PubSubEventSource(tx.Project, tx.Topic))
		event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
		event.SetType(v1alpha1.PubSubEventType)
		event.SetID(uuid.New().String())
		event.Data = msg.Data
		// TODO: this will drop the other metadata related to the the topic and subscription names.
		return &event, nil
	}
	return nil, err
}

func (a *Invoker) newPubSubClient(ctx context.Context) (cloudevents.Client, error) {
	tOpts := []cepubsub.Option{
		cepubsub.WithBinaryEncoding(),
		cepubsub.WithProjectID(a.ProjectID),
		// cepubsub.WithTopicID(a.TopicID),
		// cepubsub.WithSubscriptionID(a.SubscriptionID),
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
		cloudevents.WithConverterFn(a.convert),
	)
}
