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

	"github.com/google/knative-gcp/pkg/gclient/iam"
)

// Client matches the interface exposed by pubsub.Client
// see https://godoc.org/cloud.google.com/go/pubsub#Client
type Client interface {
	// Close see https://godoc.org/cloud.google.com/go/pubsub#Client.Close
	Close() error
	// Topic see https://godoc.org/cloud.google.com/go/pubsub#Client.Topic
	Topic(id string) Topic
	// Subscription see https://godoc.org/cloud.google.com/go/pubsub#Client.Subscription
	Subscription(id string) Subscription
	// CreateSubscription see https://godoc.org/cloud.google.com/go/pubsub#Client.CreateSubscription
	CreateSubscription(ctx context.Context, id string, cfg SubscriptionConfig) (Subscription, error)
	// CreateTopic see https://godoc.org/cloud.google.com/go/pubsub#Client.CreateTopic
	CreateTopic(ctx context.Context, id string) (Topic, error)
}

// Client matches the interface exposed by pubsub.Subscription
// see https://godoc.org/cloud.google.com/go/pubsub#Subscription
type Subscription interface {
	// Exists see https://godoc.org/cloud.google.com/go/pubsub#Subscription.Exists
	Exists(ctx context.Context) (bool, error)
	// Config see https://godoc.org/cloud.google.com/go/pubsub#Subscription.Config
	Config(ctx context.Context) (SubscriptionConfig, error)
	// Update see https://godoc.org/cloud.google.com/go/pubsub#Subscription.Update
	Update(ctx context.Context, cfg SubscriptionConfig) (SubscriptionConfig, error)
	// Delete see https://godoc.org/cloud.google.com/go/pubsub#Subscription.Delete
	Delete(ctx context.Context) error
}

// Client matches the interface exposed by pubsub.Topic
// see https://godoc.org/cloud.google.com/go/pubsub#Topic
type Topic interface {
	// Exists see https://godoc.org/cloud.google.com/go/pubsub#Topic.Exists
	Exists(ctx context.Context) (bool, error)
	// Delete see https://godoc.org/cloud.google.com/go/pubsub#Topic.Delete
	Delete(ctx context.Context) error
	// IAM see https://godoc.org/cloud.google.com/go/pubsub#Topic.IAM
	IAM() iam.Handle
}
