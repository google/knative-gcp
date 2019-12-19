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

// bucketHandle wraps storage.Bucket. Is the topic that will be used everywhere except unit tests.
type storageBucket struct {
	handle *storage.BucketHandle
}

// Verify that it satisfies the storage.Bucket interface.
var _ Bucket = &storageBucket{}

func (b *storageBucket) AddNotification(ctx context.Context, n *storage.Notification) (*storage.Notification, error) {
	return b.handle.AddNotification(ctx, n)
}

func (b *storageBucket) Notifications(ctx context.Context) (map[string]*storage.Notification, error) {
	return b.handle.Notifications(ctx)
}

func (b *storageBucket) DeleteNotification(ctx context.Context, id string) error {
	return b.handle.DeleteNotification(ctx, id)
}
