// +build wireinject

/*
Copyright 2020 Google LLC.

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

package pool

import (
	"context"

	"cloud.google.com/go/pubsub"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/wire"
)

func InitializeTestFanoutPool(
	ctx context.Context,
	targets config.ReadonlyTargets,
	pubsubClient *pubsub.Client,
	opts ...Option,
) (*FanoutPool, error) {
	panic(wire.Build(
		NewFanoutPool,
		NewDeliverClient,
		NewRetryClient,
		cehttp.New,
		wire.Value([]cehttp.Option(nil)),
		wire.Value(DefaultCEClientOpts),
	))
}

func InitializeTestRetryPool(
	targets config.ReadonlyTargets,
	pubsubClient *pubsub.Client,
	opts ...Option,
) (*RetryPool, error) {
	panic(wire.Build(
		NewRetryPool,
		NewDeliverClient,
		cehttp.New,
		wire.Value([]cehttp.Option(nil)),
		wire.Value(DefaultCEClientOpts),
	))
}
