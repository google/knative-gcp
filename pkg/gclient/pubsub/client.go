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

package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
)

// CreateFn is a factory function to create a Pub/Sub client.
type CreateFn func(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error)

// NewClient creates a new wrapped Pub/Sub client.
func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error) {
	client, err := pubsub.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, err
	}
	return &pubsubClient{
		client: client,
	}, nil
}

// pubsubClient wraps pubsub.Client. Is the client that will be used everywhere except unit tests.
type pubsubClient struct {
	client *pubsub.Client
}

// Verify that it satisfies the pubsub.Client interface.
var _ Client = &pubsubClient{}

// Close implements pubsub.Client.Close
func (c *pubsubClient) Close() error {
	return c.client.Close()
}

// Subscription implements pubsub.Client.Subscription
func (c *pubsubClient) Subscription(id string) Subscription {
	return &pubsubSubscription{sub: c.client.Subscription(id)}
}

// CreateSubscription implements pubsub.Client.CreateSubscription
func (c *pubsubClient) CreateSubscription(ctx context.Context, id string, cfg SubscriptionConfig) (Subscription, error) {
	var topic *pubsub.Topic
	if t, ok := cfg.Topic.(*pubsubTopic); ok {
		topic = t.topic
	}
	pscfg := pubsub.SubscriptionConfig{
		Topic:               topic,
		AckDeadline:         cfg.AckDeadline,
		RetainAckedMessages: cfg.RetainAckedMessages,
		RetentionDuration:   cfg.RetentionDuration,
		Labels:              cfg.Labels,
	}
	sub, err := c.client.CreateSubscription(ctx, id, pscfg)
	if err != nil {
		return nil, err
	}
	return &pubsubSubscription{sub: sub}, nil
}

// Topic implements pubsub.Client.Topic
func (c *pubsubClient) Topic(id string) Topic {
	return &pubsubTopic{topic: c.client.Topic(id)}
}

// CreateTopic implements pubsub.Client.CreateTopic
func (c *pubsubClient) CreateTopic(ctx context.Context, id string) (Topic, error) {
	topic, err := c.client.CreateTopic(ctx, id)
	if err != nil {
		return nil, err
	}
	return &pubsubTopic{topic: topic}, nil
}

// CreateTopicWithConfig implements pubsub.Client.CreateTopicWithConfig
func (c *pubsubClient) CreateTopicWithConfig(ctx context.Context, id string, cfg *pubsub.TopicConfig) (Topic, error) {
	topic, err := c.client.CreateTopicWithConfig(ctx, id, cfg)
	if err != nil {
		return nil, err
	}
	return &pubsubTopic{topic: topic}, nil
}
