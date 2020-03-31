/*
Copyright 2020 Google LLC

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

package context

import (
	"context"

	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/broker/config"
)

type brokerKeyKey struct{}
type brokerKey struct{}

// WithBrokerKey sets a broker key in the context.
func WithBrokerKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, brokerKeyKey{}, key)
}

// GetBrokerKey gets the broker key from the context.
func GetBrokerKey(ctx context.Context) string {
	untyped := ctx.Value(brokerKeyKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic("Unable to fetch Broker key from context.")
	}
	return untyped.(string)
}

// WithBroker sets a broker in the context.
func WithBroker(ctx context.Context, b *config.Broker) context.Context {
	return context.WithValue(ctx, brokerKey{}, b)
}

// GetBroker gets a broker from the context.
func GetBroker(ctx context.Context) *config.Broker {
	untyped := ctx.Value(brokerKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic("Unable to fetch Broker from context.")
	}
	return untyped.(*config.Broker)
}
