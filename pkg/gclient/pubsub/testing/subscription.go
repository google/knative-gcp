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

// testSubscription is a test Pub/Sub subscription.
type testSubscription struct {
	data TestSubscriptionData
	id   string
}

// TestSubscriptionData is the data used to configure the test Subscription.
type TestSubscriptionData struct {
	ExistsErr error
	Exists    bool
	ConfigErr error
	UpdateErr error
	DeleteErr error
}

// Verify that it satisfies the pubsub.Subscription interface.
var _ pubsub.Subscription = &testSubscription{}

// Exists implements Subscription.Exists.
func (s *testSubscription) Exists(ctx context.Context) (bool, error) {
	return s.data.Exists, s.data.ExistsErr
}

// Config implements Subscription.Config.
func (s *testSubscription) Config(ctx context.Context) (pubsub.SubscriptionConfig, error) {
	return pubsub.SubscriptionConfig{}, s.data.ConfigErr
}

// Update implements Subscription.Update.
func (s *testSubscription) Update(ctx context.Context, cfg pubsub.SubscriptionConfig) (pubsub.SubscriptionConfig, error) {
	return cfg, s.data.UpdateErr
}

// Delete implements Subscription.Delete.
func (s *testSubscription) Delete(ctx context.Context) error {
	return s.data.DeleteErr
}

func (s *testSubscription) ID() string {
	return s.id
}
