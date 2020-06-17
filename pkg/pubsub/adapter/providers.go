package adapter

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/wire"
)

// TODO Name and ResourceGroup might cause problems in the near future, as we might use a single receive-adapter
//  for multiple source objects. Same with Namespace, when doing multi-tenancy.
type Name string
type Namespace string
type ResourceGroup string
type AdapterType string

type SinkURI string
type TransformerURI string
type SubscriptionID string

// AdapterSet provides an adapter with a PubSub client and HTTP client.
var AdapterSet wire.ProviderSet = wire.NewSet(
	NewAdapter,
	clients.NewPubsubClient,
	NewPubSubSubscription,
	NewStatsReporter,
	clients.NewHTTPClient,
)

func NewPubSubSubscription(ctx context.Context, client *pubsub.Client, subscriptionID SubscriptionID) *pubsub.Subscription {
	return client.Subscription(string(subscriptionID))
}
