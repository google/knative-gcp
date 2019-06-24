package pubsub

import (
	"context"
)

type Client interface {
	// Client
	Close() error

	// Topic
	Topic(id string) Topic

	// Subscription
	Subscription(id string) Subscription
	CreateSubscription(ctx context.Context, id string, cfg SubscriptionConfig) (Subscription, error)
}

type Subscription interface {
	Exists(ctx context.Context) (bool, error)
	Config(ctx context.Context) (SubscriptionConfig, error)
	Delete(ctx context.Context) error
}

type Topic interface {
	Exists(ctx context.Context) (bool, error)
}
