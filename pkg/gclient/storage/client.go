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

package storage

import (
	"context"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// CreateFn is a factory function to create a Storage client.
type CreateFn func(ctx context.Context, opts ...option.ClientOption) (Client, error)

// NewClient creates a new wrapped Storage client.
func NewClient(ctx context.Context, opts ...option.ClientOption) (Client, error) {
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &storageClient{
		client: client,
	}, nil
}

// storageClient wraps storage.Client. Is the client that will be used everywhere except unit tests.
type storageClient struct {
	client *storage.Client
}

// Verify that it satisfies the storage.Client interface.
var _ Client = &storageClient{}

// Close implements storage.Client.Close
func (c *storageClient) Close() error {
	return c.client.Close()
}

// Bucket implements storage.Client.Bucket
func (c *storageClient) Bucket(name string) Bucket {
	return &storageBucket{handle: c.client.Bucket(name)}
}
