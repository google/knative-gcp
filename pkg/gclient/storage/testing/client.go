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

	"github.com/google/knative-gcp/pkg/gclient/storage"
	"google.golang.org/api/option"
)

// TestClientCreator returns a pubsub.CreateFn used to construct the test Storage client.
func TestClientCreator(value interface{}) storage.CreateFn {
	var data TestClientData
	var ok bool
	if data, ok = value.(TestClientData); !ok {
		data = TestClientData{}
	}
	if data.CreateClientErr != nil {
		return func(ctx context.Context, opts ...option.ClientOption) (storage.Client, error) {
			return nil, data.CreateClientErr
		}
	}

	return func(ctx context.Context, opts ...option.ClientOption) (storage.Client, error) {
		return &testClient{
			data: data,
		}, nil
	}
}

// TestClientData is the data used to configure the test Storage client.
type TestClientData struct {
	CreateClientErr       error
	CreateSubscriptionErr error
	CreateTopicErr        error
	CloseErr              error
	BucketData            TestBucketData
}

// testClient is a test Storage client.
type testClient struct {
	data TestClientData
}

// Verify that it satisfies the storage.Client interface.
var _ storage.Client = &testClient{}

// Close implements client.Close
func (c *testClient) Close() error {
	return c.data.CloseErr
}

// Bucket implements client.Bucket
func (c *testClient) Bucket(name string) storage.Bucket {
	return &testBucket{data: c.data.BucketData}
}
