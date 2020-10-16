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

package clients

import (
	"context"
	nethttp "net/http"
	"time"

	"cloud.google.com/go/pubsub"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

type Port int
type ProjectID string
type MaxConnsPerHost int

// NewHTTPMessageReceiver wraps kncloudevents.NewHttpMessageReceiver with type-safe options.
func NewHTTPMessageReceiver(port Port) *kncloudevents.HTTPMessageReceiver {
	return kncloudevents.NewHTTPMessageReceiver(int(port))
}

// NewPubsubClient provides a pubsub client from PubsubClientOpts.
func NewPubsubClient(ctx context.Context, projectID ProjectID) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, string(projectID))
}

// NewObservedPubsubClient creates a pubsub Cloudevents client with observability support.
func NewObservedPubsubClient(ctx context.Context, client *pubsub.Client) (cev2.Client, error) {
	p, err := cepubsub.New(ctx, cepubsub.WithClient(client))
	if err != nil {
		return nil, err
	}

	// Use the pubsub prototol to make a new CloudEvents client.
	return cev2.NewClientObserved(p,
		cev2.WithUUIDs(),
		cev2.WithTimeNow(),
		cev2.WithTracePropagation,
	)
}

func NewHTTPClient(_ context.Context, maxConnsPerHost MaxConnsPerHost) *nethttp.Client {
	return &nethttp.Client{
		Transport: &ochttp.Transport{
			Base: &nethttp.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: int(maxConnsPerHost),
				MaxConnsPerHost:     int(maxConnsPerHost),
				IdleConnTimeout:     30 * time.Second,
			},
			Propagation: tracecontextb3.TraceContextEgress,
		},
	}
}
