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

package testing

import (
	"context"

	. "cloud.google.com/go/storage"
	"github.com/google/knative-gcp/pkg/gclient/storage"
)

// testBucket is a test Storage bucket.
type testBucket struct {
	data TestBucketData
}

// TestBucketData is the data used to configure the test Bucket.
type TestBucketData struct {
	Notifications      map[string]*Notification
	NotificationsErr   error
	AddNotificationErr error
	DeleteErr          error
}

// Verify that it satisfies the storage.Bucket interface.
var _ storage.Bucket = &testBucket{}

// AddNotification implements bucket.AddNotification
func (b *testBucket) AddNotification(ctx context.Context, n *Notification) (*Notification, error) {
	if b.data.AddNotificationErr != nil {
		return nil, b.data.AddNotificationErr
	}
	return n, nil
}

// Notifications implements bucket.Notifications
func (b *testBucket) Notifications(ctx context.Context) (map[string]*Notification, error) {
	return b.data.Notifications, b.data.NotificationsErr
}

// DeleteNotification implements bucket.DeleteNotification
func (b *testBucket) DeleteNotification(ctx context.Context, id string) error {
	return b.data.DeleteErr
}
