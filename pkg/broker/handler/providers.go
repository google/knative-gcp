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

package handler

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"github.com/google/wire"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

var (
	DefaultCEClientOpts = []ceclient.Option{
		ceclient.WithUUIDs(),
		ceclient.WithTimeNow(),
		ceclient.WithTracePropagation(),
	}

	DefaultHTTPClient = &http.Client{
		Transport: &ochttp.Transport{
			Base: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 500,
				MaxConnsPerHost:     500,
				IdleConnTimeout:     30 * time.Second,
			},
			Propagation: tracecontextb3.TraceContextEgress,
		},
	}

	// ProviderSet provides the fanout and retry sync pools using the default client options. In
	// order to inject either pool, ProjectID, []Option, and config.ReadOnlyTargets must be
	// externally provided.
	ProviderSet = wire.NewSet(
		NewFanoutPool,
		NewRetryPool,
		NewPubsubClient,
		NewRetryClient,
		wire.Value(DefaultHTTPClient),
		wire.Value(DefaultCEClientOpts),
	)
)

type (
	ProjectID   string
	RetryClient ceclient.Client
)

// NewPubsubClient provides a pubsub client for the supplied project ID.
func NewPubsubClient(ctx context.Context, projectID ProjectID) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, string(projectID))
}

// NewRetryClient provides a retry CE client from a PubSub client and list of CE client options.
func NewRetryClient(ctx context.Context, client *pubsub.Client, opts ...ceclient.Option) (RetryClient, error) {
	rps, err := cepubsub.New(ctx, cepubsub.WithClient(client))
	if err != nil {
		return nil, err
	}

	return ceclient.NewObserved(rps, opts...)
}
