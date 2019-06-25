/*
Copyright 2019 Google LLC

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

package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
)

// NewClient creates a new wrapped Pub/Sub client.
func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error) {
	client, err := pubsub.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, err
	}
	return &pubsubClient{
		client: client,
	}, nil
}

// pubsubClient wraps pubsub.Client.
type pubsubClient struct {
	client *pubsub.Client
}

// Close implements pubsub.Client.Close
func (c *pubsubClient) Close() error {
	return c.client.Close()
}
