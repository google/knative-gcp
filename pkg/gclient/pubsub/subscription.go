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

// SubscriptionConfig re-implements pubsub.SubscriptionConfig to allow us to
// use a wrapped Topic internally.
type SubscriptionConfig struct {
	Topic               Topic
	AckDeadline         time.Duration
	RetainAckedMessages bool
	RetentionDuration   time.Duration
	Labels              map[string]string
}

// pubsubSubscription wraps pubsub.Subscription. Is the subscription that will be used everywhere except unit tests.
type pubsubSubscription struct {
	sub *pubsub.Subscription
}

// Verify that it satisfies the pubsub.Subscription interface.
var _ Subscription = &pubsubSubscription{}

// Exists implements pubsub.Subscription.Exists
func (s *pubsubSubscription) Exists(ctx context.Context) (bool, error) {
	return s.sub.Exists(ctx)
}

// Config implements pubsub.Subscription.Config
func (s *pubsubSubscription) Config(ctx context.Context) (SubscriptionConfig, error) {
	cfg, err := s.sub.Config(ctx)
	if err != nil {
		return SubscriptionConfig{}, err
	}
	return SubscriptionConfig{
		Topic:               &pubsubTopic{topic: cfg.Topic},
		AckDeadline:         cfg.AckDeadline,
		RetainAckedMessages: cfg.RetainAckedMessages,
		RetentionDuration:   cfg.RetentionDuration,
		Labels:              cfg.Labels,
	}, nil
}

// Update implements pubsub.Subscription.Update
func (s *pubsubSubscription) Update(ctx context.Context, cfg SubscriptionConfig) (SubscriptionConfig, error) {
	config := pubsub.SubscriptionConfigToUpdate{
		Labels:              cfg.Labels,
		RetainAckedMessages: cfg.RetainAckedMessages,
		RetentionDuration:   cfg.RetentionDuration,
		AckDeadline:         cfg.AckDeadline,
	}
	updatedConfig, err := s.sub.Update(ctx, config)
	if err != nil {
		return SubscriptionConfig{}, err
	}
	return SubscriptionConfig{
		Topic:               &pubsubTopic{topic: updatedConfig.Topic},
		AckDeadline:         updatedConfig.AckDeadline,
		RetainAckedMessages: updatedConfig.RetainAckedMessages,
		RetentionDuration:   updatedConfig.RetentionDuration,
		Labels:              updatedConfig.Labels,
	}, err
}

// Delete implements pubsub.Subscription.Delete
func (s *pubsubSubscription) Delete(ctx context.Context) error {
	return s.sub.Delete(ctx)
}

// ID implements pubsub.Subscription.ID
func (s *pubsubSubscription) ID() string {
	return s.sub.ID()
}
