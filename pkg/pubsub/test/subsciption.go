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
