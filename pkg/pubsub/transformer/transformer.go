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

package transformer

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
)

// PushMessage represents the format Pub/Sub uses to push events.
type PushMessage struct {
	// Subscription is the subscription ID that received this Message.
	Subscription string `json:"subscription"`
	// Message holds the Pub/Sub message contents.
	Message *PubSubMessage `json:"message,omitempty"`
}

type PubSubMessage struct {
	// ID identifies this message.
	// This ID is assigned by the server and is populated for Messages obtained from a subscription.
	// This field is read-only.
	ID string `json:"id,omitempty"`

	// Data is the actual data in the message.
	Data []byte `json:"data,omitempty"`

	// Attributes represents the key-value pairs the current message
	// is labelled with.
	Attributes map[string]string `json:"attributes,omitempty"`

	// The time at which the message was published.
	// This is populated by the server for Messages obtained from a subscription.
	// This field is read-only.
	PublishTime time.Time `json:"publish_time,omitempty"`
}

// Transformer implements the Pub/Sub transformer to convert generic Cloud
// Pub/Sub events into compatible formats.
type Transformer struct {
	// client is the cloudevents client to use to receive events.
	client cloudevents.Client
}

func (a *Transformer) Start(ctx context.Context) error {
	var err error

	// Receive Events on Pub/Sub.
	if a.client == nil {
		if a.client, err = kncloudevents.NewDefaultClient(); err != nil {
			return fmt.Errorf("failed to create cloudevent client: %s", err.Error())
		}
	}

	return a.client.StartReceiver(ctx, a.receive)
}

func (a *Transformer) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))

	// Convert data to PushMessage.

	push := cloudevents.NewEvent(event.SpecVersion())
	push.Context = event.Context.Clone()

	attrs := map[string]string{}
	var s string
	if err := event.ExtensionAs("attributes", &s); err != nil {
		logger.Warnw("Failed to get attributes extensions.", zap.Error(err))
	} else {
		logger.Info("attributes:", zap.String("s", s))
		// TODO
	}

	logger.Info("fyi, attributes:", zap.Any("event.attrs", event.Extensions()["attributes"]))

	msg := &PubSubMessage{
		ID:          event.ID(),
		Attributes:  attrs,
		PublishTime: event.Time(),
	}

	var subscription string
	if sub, ok := event.Extensions()["subscription"]; ok {
		subscription = sub.(string)
	}

	logger.Info("Event:", zap.String("event", event.String())) // TODO: remove.
	logger.Info("Data:", zap.Any("data", event.Data))          // TODO: remove.

	//if event.DataEncoded {
	//	push.DataEncoded = true
	//	msg.Data = event.Data.([]byte)
	//} else if event.Data != nil {

	//var err error
	//msg.Data, err = event.DataBytes()
	var raw json.RawMessage
	if err := event.DataAs(&raw); err != nil {
		logger.Warnw("Failed to get data as raw json.", zap.Error(err))
	} else {
		msg.Data = raw
		logger.Info("Success getting json raw.", zap.Any("raw", raw))
	}
	//
	//if err := event.DataAs(&msg.Data); err != nil {
	//	logger.Warnw("Failed to get data.", zap.Error(err))
	//}
	//}

	if err := push.SetData(&PushMessage{
		Subscription: subscription,
		Message:      msg,
	}); err != nil {
		logger.Warnw("Failed to set data.", zap.Error(err))
	}

	resp.RespondWith(200, &push)
	return nil
}
