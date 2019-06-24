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
	"time"

	"cloud.google.com/go/pubsub"
)

func (c *PubSubClient) Subscription(id string) Subscription {
	return &PubSubSubscription{sub: c.client.Subscription(id)}
}

func (c *PubSubClient) CreateSubscription(ctx context.Context, id string, cfg SubscriptionConfig) (Subscription, error) {
	var topic *pubsub.Topic
	if t, ok := cfg.Topic.(*PubSubTopic); ok {
		topic = t.topic
	}
	pscfg := pubsub.SubscriptionConfig{
		Topic:               topic,
		AckDeadline:         cfg.AckDeadline,
		RetainAckedMessages: cfg.RetainAckedMessages,
		RetentionDuration:   cfg.RetentionDuration,
		Labels:              cfg.Labels,
	}
	if sub, err := c.client.CreateSubscription(ctx, id, pscfg); err != nil {
		return nil, err
	} else {
		return &PubSubSubscription{sub: sub}, nil
	}
}

type SubscriptionConfig struct {
	Topic               Topic
	AckDeadline         time.Duration
	RetainAckedMessages bool
	RetentionDuration   time.Duration
	Labels              map[string]string
}

type PubSubSubscription struct {
	sub *pubsub.Subscription
}

func (s *PubSubSubscription) Exists(ctx context.Context) (bool, error) {
	return s.sub.Exists(ctx)
}

func (s *PubSubSubscription) Config(ctx context.Context) (SubscriptionConfig, error) {
	if cfg, err := s.sub.Config(ctx); err != nil {
		return SubscriptionConfig{}, err
	} else {
		return SubscriptionConfig{
			Topic:               &PubSubTopic{topic: cfg.Topic},
			AckDeadline:         cfg.AckDeadline,
			RetainAckedMessages: cfg.RetainAckedMessages,
			RetentionDuration:   cfg.RetentionDuration,
			Labels:              cfg.Labels,
		}, nil
	}
}

func (s *PubSubSubscription) Delete(ctx context.Context) error {
	return s.sub.Delete(ctx)
}
