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

package v1

import (
	"fmt"
	"time"
)

const (
	CloudPubSubMessagePublishedEventType = "google.cloud.pubsub.topic.v1.messagePublished"
	CloudPubSubEventDataSchema           = "https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/pubsub/v1/data.proto"
)

// CloudPubSubEventSource returns the Cloud Pub/Sub CloudEvent source value.
func CloudPubSubEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/projects/%s/topics/%s", googleCloudProject, topic)
}

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
