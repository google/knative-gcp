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
)

// Client matches the interface exposed by storage.Client
// see https://godoc.org/cloud.google.com/go/storage#Client
type Client interface {
	// Close see https://godoc.org/cloud.google.com/go/storage#Client.Close
	Close() error
	// Bucket see https://godoc.org/cloud.google.com/go/storage#Client.Bucket
	Bucket(name string) Bucket
}

// Bucket matches the interface exposed by storage.BucketHandle
// see https://godoc.org/cloud.google.com/go/storage#BucketHandle
type Bucket interface {
	// AddNotification see https://godoc.org/cloud.google.com/go/storage#BucketHandle.AddNotification
	AddNotification(ctx context.Context, n *storage.Notification) (*storage.Notification, error)
	// Notifications see https://godoc.org/cloud.google.com/go/storage#BucketHandle.Notifications
	Notifications(ctx context.Context) (map[string]*storage.Notification, error)
	// DeleteNotification see https://godoc.org/cloud.google.com/go/storage#BucketHandle.DeleteNotification
	DeleteNotification(ctx context.Context, id string) error
}
