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

package ingress

import (
	"context"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"knative.dev/eventing/pkg/kncloudevents"
)

type Args struct {
	IngressPort   int
	ProjectID     string
	PodName       string
	ContainerName string
}

// NewHTTPMessageReceiver wraps kncloudevents.NewHttpMessageReceiver with type-safe options.
func NewHTTPMessageReceiver(args Args) *kncloudevents.HttpMessageReceiver {
	return kncloudevents.NewHttpMessageReceiver(args.IngressPort)
}

// NewPubsubClient provides a pubsub client from PubsubClientOpts.
func NewPubsubClient(ctx context.Context, args Args) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, args.ProjectID)
}

// NewPubsubDecoupleClient creates a pubsub Cloudevents client to use to publish events to decouple queues.
func NewPubsubDecoupleClient(ctx context.Context, client *pubsub.Client) (cev2.Client, error) {
	// Make a pubsub protocol for the CloudEvents client.
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
