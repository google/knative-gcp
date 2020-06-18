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
	"time"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
)

func convertCloudPubSub(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(msg.ID)
	event.SetTime(msg.PublishTime)

	project, err := GetProjectKey(ctx)
	if err != nil {
		return nil, err
	}
	topic, err := GetTopicKey(ctx)
	if err != nil {
		return nil, err
	}

	event.SetSource(v1beta1.CloudPubSubSourceEventSource(project, topic))
	event.SetType(v1beta1.CloudPubSubSourcePublish)

	subscription, err := GetSubscriptionKey(ctx)
	if err != nil {
		return nil, err
	}

	pushMessage := &PushMessage{
		Subscription: subscription,
		Message: &PubSubMessage{
			ID:          msg.ID,
			Attributes:  msg.Attributes,
			PublishTime: msg.PublishTime,
			Data:        msg.Data,
		},
	}

	if err := event.SetData(cev2.ApplicationJSON, pushMessage); err != nil {
		return nil, err
	}
	return &event, nil
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
