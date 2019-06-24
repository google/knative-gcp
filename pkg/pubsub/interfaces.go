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
