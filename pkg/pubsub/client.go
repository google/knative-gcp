package pubsub

import (
	"cloud.google.com/go/pubsub"
	"context"
	"google.golang.org/api/option"
)

func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error) {
	if client, err := pubsub.NewClient(ctx, projectID, opts...); err != nil {
		return nil, err
	} else {
		return &PubSubClient{
			client: client,
		}, nil
	}
}

type PubSubClient struct {
	client *pubsub.Client
}

func (c *PubSubClient) Close() error {
	return c.client.Close()
}
