package test

import (
	"context"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub"
)

func (c *TestClient) Topic(id string) pubsub.Topic {
	return &TestTopic{id: id}
}

type TestTopic struct {
	id string
}

func (t *TestTopic) Exists(ctx context.Context) (bool, error) {
	return true, nil
}
