package test

import (
	"cloud.google.com/go/pubsub"
	"context"
	"google.golang.org/api/option"
)

func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (pubsub.Client, error) {
	return &TestClient{Project: projectID}, nil
}

type TestClient struct {
	Project string
}

func (c *TestClient) Close() error {
	return nil
}
