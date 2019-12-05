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

package testing

import (
	"context"

	"github.com/google/knative-gcp/pkg/gclient/pubsub"
)

// TestSubscription is a test Pub/Sub subscription.
type TestSubscription struct {
	id string
}

// Verify that it satisfies the pubsub.Subscription interface.
var _ pubsub.Subscription = &TestSubscription{}

// Exists implements Subscription.Exists.
func (s *TestSubscription) Exists(ctx context.Context) (bool, error) {
	return true, nil
}

// Config implements Subscription.Config.
func (s *TestSubscription) Config(ctx context.Context) (pubsub.SubscriptionConfig, error) {
	return pubsub.SubscriptionConfig{}, nil
}

// Update implements Subscription.Update.
func (s *TestSubscription) Update(ctx context.Context, cfg pubsub.SubscriptionConfig) (pubsub.SubscriptionConfig, error) {
	return cfg, nil
}

// Delete implements Subscription.Delete.
func (s *TestSubscription) Delete(ctx context.Context) error {
	return nil
}
