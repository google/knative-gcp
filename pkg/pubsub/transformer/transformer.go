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
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"knative.dev/pkg/logging"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/kncloudevents"
)

// PushMessage represents the format Pub/Sub uses to push events.
type PushMessage struct {
	// Subscription is the subscription ID that received this Message.
	Subscription string `json:"subscription"`
	// Message holds the Pub/Sub message contents.
	Message *PubSubMessage `json:"message,omitempty"`
}

// PubSubMessage matches the inner message format used by Push Subscriptions.
type PubSubMessage struct {
	// ID identifies this message. This ID is assigned by the server and is
	// populated for Messages obtained from a subscription.
	// This field is read-only.
	ID string `json:"id,omitempty"`

	// Data is the actual data in the message.
	Data interface{} `json:"data,omitempty"`

	// Attributes represents the key-value pairs the current message
	// is labelled with.
	Attributes map[string]string `json:"attributes,omitempty"`

	// The time at which the message was published. This is populated by the
	// server for Messages obtained from a subscription.
	// This field is read-only.
	PublishTime time.Time `json:"publish_time,omitempty"`
}

// Transformer implements the Pub/Sub transformer to convert generic Cloud
// Pub/Sub events into compatible formats.
type Transformer struct {
	// client is the cloudevents client to use to receive events.
	client cloudevents.Client
}

// Start starts the adapter. Note: Only call once, not thread safe.
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

	if event.Type() != v1alpha1.PubSubEventType {
		logger.Warnw("Not attempting to convert non-Pub/Sub type.", zap.String("type", event.Type()))
		resp.RespondWith(http.StatusNotModified, &event)
		return nil
	}

	// Convert data to PushMessage.

	push := cloudevents.NewEvent(event.SpecVersion())
	push.Context = event.Context.Clone()

	// This is a workaround with the current CloudEvents SDK, it does not
	// handle extensions very well. The following code converts the
	// "attributes" extension into a attributes map that Pub/Sub push
	// consumers expect.
	attrs := make(map[string]string, 0)
	if at, ok := event.Extensions()["attributes"].(map[string]interface{}); ok {
		for k, v := range at {
			attrs[k] = v.(string)
		}
	} else {
		logger.Warnw("Attributes not found.")
	}

	msg := &PubSubMessage{
		ID:          event.ID(),
		Attributes:  attrs,
		PublishTime: event.Time(),
	}

	var subscription string
	if sub, ok := event.Extensions()["subscription"]; ok {
		subscription = sub.(string)
	}

	var raw json.RawMessage
	if err := event.DataAs(&raw); err != nil {
		logger.Debugw("Failed to get data as raw json, using as is.", zap.Error(err))
		// Use data as a byte slice.
		msg.Data = event.Data
	} else {
		// Use data as a raw message.
		msg.Data = raw
	}

	if err := push.SetData(&PushMessage{
		Subscription: subscription,
		Message:      msg,
	}); err != nil {
		logger.Warnw("Failed to set data.", zap.Error(err))
	}

	resp.RespondWith(http.StatusOK, &push)
	return nil
}
