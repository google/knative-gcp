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

package converters

import (
	"context"
	"encoding/json"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	. "github.com/cloudevents/sdk-go/pkg/cloudevents"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

func convertPubSub(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	event.SetSource(v1alpha1.CloudPubSubSourceEventSource(tx.Project, tx.Topic))
	// We do not know the content type and we do not want to inspect the payload,
	// thus we set this generic one.
	event.SetDataContentType("application/octet-stream")
	event.SetType(v1alpha1.CloudPubSubSourcePublish)
	// Set the schema if it comes as an attribute.
	if val, ok := msg.Attributes["schema"]; ok {
		delete(msg.Attributes, "schema")
		event.SetDataSchema(val)
	}
	// Set the mode to be an extension attribute.
	event.SetExtension("knativecemode", string(sendMode))
	event.Data = msg.Data
	event.DataEncoded = true

	// If send mode is Push, convert to Pub/Sub Push payload style.
	if sendMode == Push {
		logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))
		// set the content type to something that can be handled by codec.go
		event.SetDataContentType(cloudevents.ApplicationJSON)
		msg := &pubSubMessage{
			ID:          event.ID(),
			Attributes:  msg.Attributes,
			PublishTime: event.Time(),
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

		if err := event.SetData(&pushMessage{
			Subscription: tx.Subscription,
			Message:      msg,
		}); err != nil {
			logger.Warnw("Failed to set data.", zap.Error(err))
		}
	} else if msg.Attributes != nil && len(msg.Attributes) > 0 {
		// Attributes are promoted to extensions if send mode is not Push.
		for k, v := range msg.Attributes {
			// CloudEvents v1.0 attributes MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9') as per
			// the spec. It's not even possible for a conformant transport to allow non-base36 characters.
			// Note `SetExtension` will make it lowercase so only `IsAlphaNumeric` needs to be checked here.
			if IsAlphaNumeric(k) {
				event.SetExtension(k, v)
			}
		}

	}
	return &event, nil
}

// pushMessage represents the format Pub/Sub uses to push events.
type pushMessage struct {
	// Subscription is the subscription ID that received this Message.
	Subscription string `json:"subscription"`
	// Message holds the Pub/Sub message contents.
	Message *pubSubMessage `json:"message,omitempty"`
}

// PubSubMessage matches the inner message format used by Push Subscriptions.
type pubSubMessage struct {
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
