package publisher

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/wire"
	"knative.dev/eventing/pkg/kncloudevents"
)

type TopicID string

// PublisherSet provides a handler with a real HTTPMessageReceiver and a PubSub client.
var PublisherSet wire.ProviderSet = wire.NewSet(
	NewPublisher,
	clients.NewHTTPMessageReceiver,
	clients.NewPubsubClient,
	NewPubSubTopic,
	wire.Bind(new(HttpMessageReceiver), new(*kncloudevents.HTTPMessageReceiver)),
)

// NewPubSubTopic provides a pubsub topic from a PubSub client.
func NewPubSubTopic(ctx context.Context, client *pubsub.Client, topicID TopicID) *pubsub.Topic {
	return client.Topic(string(topicID))
}
