package test

import (
	"context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub"
)

func (c *TestClient) Subscription(id string) pubsub.Subscription {
	return &TestSubscription{id: id}
}

func (c *TestClient) CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (pubsub.Subscription, error) {
	return &TestSubscription{id: id}, nil
}

type TestSubscription struct {
	id string
}

func (s *TestSubscription) Exists(ctx context.Context) (bool, error) {
	return true, nil
}

func (s *TestSubscription) Config(ctx context.Context) (pubsub.SubscriptionConfig, error) {
	return pubsub.SubscriptionConfig{}, nil
}

func (s *TestSubscription) Delete(ctx context.Context) error {
	return nil
}
