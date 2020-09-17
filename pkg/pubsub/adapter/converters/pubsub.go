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

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
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

	event.SetSource(schemasv1.CloudPubSubEventSource(project, topic))
	event.SetType(schemasv1.CloudPubSubMessagePublishedEventType)
	event.SetDataSchema(schemasv1.CloudPubSubEventDataSchema)
	subscription, err := GetSubscriptionKey(ctx)
	if err != nil {
		return nil, err
	}

	pushMessage := &schemasv1.PushMessage{
		Subscription: subscription,
		Message: &schemasv1.PubSubMessage{
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
