package adapter

import (
	"context"
	"encoding/json"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
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

// ConvertToPush convert an event to a Pub/Sub style Push payload.
func ConvertToPush(ctx context.Context, event cloudevents.Event) cloudevents.Event {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))

	tx := cepubsub.TransportContextFrom(ctx)

	push := cloudevents.NewEvent(event.SpecVersion())
	push.Context = event.Context.Clone()

	// Grab all extensions as a string, set them as attributes payload.
	attrs := make(map[string]string, 0)
	for k, _ := range event.Extensions() {
		var v string
		if err := event.ExtensionAs(k, &v); err != nil {
			attrs[k] = v
		}
	}

	msg := &PubSubMessage{
		ID:          event.ID(),
		Attributes:  attrs,
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

	if err := push.SetData(&PushMessage{
		Subscription: tx.Subscription,
		Message:      msg,
	}); err != nil {
		logger.Warnw("Failed to set data.", zap.Error(err))
	}

	return push
}
