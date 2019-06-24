package pubsub

import (
	"cloud.google.com/go/pubsub"
	"context"
)

func (c *PubSubClient) Topic(id string) Topic {
	return &PubSubTopic{topic: c.client.Topic(id)}
}

type PubSubTopic struct {
	topic *pubsub.Topic
}

func (t *PubSubTopic) Exists(ctx context.Context) (bool, error) {
	return t.topic.Exists(ctx)
}
