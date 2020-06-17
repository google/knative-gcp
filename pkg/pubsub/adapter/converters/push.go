package converters

import "time"

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
	ID string `json:"messageId,omitempty"`

	// Data is the actual data in the message.
	Data interface{} `json:"data,omitempty"`

	// Attributes represents the key-value pairs the current message
	// is labelled with.
	Attributes map[string]string `json:"attributes,omitempty"`

	// The time at which the message was published. This is populated by the
	// server for Messages obtained from a subscription.
	// This field is read-only.
	PublishTime time.Time `json:"publishTime,omitempty"`
}
