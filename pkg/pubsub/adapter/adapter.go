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

package adapter

import (
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"github.com/google/uuid"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
)

// Adapter implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Adapter struct {
	// ProjectID is the pre-existing eventing project id to use.
	ProjectID string
	// TopicID is the pre-existing eventing pub/sub topic id to use.
	TopicID string
	// SubscriptionID is the pre-existing eventing pub/sub subscription id to use.
	SubscriptionID string
	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string
	// TransformerURI is the URI messages will be forwarded on to for any transformation
	// before they are sent to SinkURI.
	TransformerURI string

	// SendMode describes how the adapter sends events. Default: Binary
	SendMode ModeType

	// inbound is the cloudevents client to use to receive events.
	inbound cloudevents.Client

	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client

	// transformer is the cloudevents client to transform received events before sending.
	transformer cloudevents.Client
}

// ModeType is the type for mode enum.
type ModeType string

const (
	// Binary mode is binary encoding.
	Binary ModeType = "binary"
	// Structured mode is structured encoding.
	Structured ModeType = "structured"
	// Default
	DefaultSendMode = Binary
)

func (a *Adapter) Start(ctx context.Context) error {
	var err error

	if a.SendMode == "" {
		a.SendMode = DefaultSendMode
	}

	// Receive Events on Pub/Sub.
	if a.inbound == nil {
		if a.inbound, err = a.newPubSubClient(ctx); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %s", err.Error())
		}
	}

	// Send events on HTTP.
	if a.outbound == nil {
		if a.outbound, err = a.newHTTPClient(a.SinkURI); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %s", err.Error())
		}
	}

	// Make the transformer client in case the TransformerURI has been set.
	if a.TransformerURI != "" {
		if a.transformer == nil {
			if a.transformer, err = kncloudevents.NewDefaultClient(a.TransformerURI); err != nil {
				return fmt.Errorf("failed to create transformer cloudevent client: %s", err.Error())
			}
		}
	}

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Adapter) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()), zap.Any("sink", a.SinkURI))

	// If a transformer has been configured, then transform the message.
	if a.transformer != nil {
		// TODO: I do not like the transformer as it is. It would be better to pass the transport context and the
		// message to the transformer function as a transform request. Better yet, only do it for conversion issues?
		transformedEvent, err := a.transformer.Send(ctx, event)
		if err != nil {
			logger.Errorf("error transforming cloud event %q", event.ID())
			return err
		}
		if transformedEvent == nil {
			logger.Warnf("cloud event %q was not transformed", event.ID())
			return nil
		}
		// Update the event with the transformed one.
		event = *transformedEvent
	}

	if r, err := a.outbound.Send(ctx, event); err != nil {
		return err
	} else if r != nil {
		resp.RespondWith(200, r)
	}

	return nil
}

func (a *Adapter) convert(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
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
		event.SetExtension("attributes", msg.Attributes)
		event.SetExtension("topic", tx.Topic)
		event.SetExtension("subscription", tx.Subscription)

		logging.FromContext(ctx).Info("Data: ", zap.String("data", string(msg.Data)))
		event.Data = msg.Data
		event.SetDataContentEncoding(cloudevents.Base64) // pubsub is giving us base64 encoded messages.
		return &event, nil
	}
	return nil, err
}

func (a *Adapter) newPubSubClient(ctx context.Context) (cloudevents.Client, error) {
	tOpts := []cepubsub.Option{
		cepubsub.WithProjectID(a.ProjectID),
		cepubsub.WithTopicID(a.TopicID),
		cepubsub.WithSubscriptionID(a.SubscriptionID),
	}

	// Make a pubsub transport for the CloudEvents client.
	t, err := cepubsub.New(ctx, tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	return cloudevents.NewClient(t,
		cloudevents.WithConverterFn(a.convert),
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)
}

func (a *Adapter) newHTTPClient(target string) (cloudevents.Client, error) {
	tOpts := []http.Option{
		cloudevents.WithTarget(target),
	}

	switch a.SendMode {
	case Binary:
		tOpts = append(tOpts, cloudevents.WithBinaryEncoding())
	case Structured:
		tOpts = append(tOpts, cloudevents.WithStructuredEncoding())
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTPTransport(tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	c, err := cloudevents.NewClient(t,
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
}
