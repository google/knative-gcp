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

	"cloud.google.com/go/pubsub"
	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"google.golang.org/api/option"
)

// TestClientCreator returns a pubsub.CreateFn used to construct the test Pub/Sub client.
func TestClientCreator(value interface{}) gpubsub.CreateFn {
	var data TestClientData
	var ok bool
	if data, ok = value.(TestClientData); !ok {
		data = TestClientData{}
	}
	if data.CreateClientErr != nil {
		return func(ctx context.Context, projectID string, opts ...option.ClientOption) (gpubsub.Client, error) {
			return nil, data.CreateClientErr
		}
	}

	return func(ctx context.Context, projectID string, opts ...option.ClientOption) (gpubsub.Client, error) {
		return &testClient{
			data: data,
		}, nil
	}
}

// TestClientData is the data used to configure the test Pub/Sub client.
type TestClientData struct {
	CreateClientErr       error
	CreateSubscriptionErr error
	CreateTopicErr        error
	CloseErr              error
	TopicData             TestTopicData
	SubscriptionData      TestSubscriptionData
	HandleData            testiam.TestHandleData
}

// testClient is a test Pub/Sub client.
type testClient struct {
	data TestClientData
}

// Verify that it satisfies the pubsub.Client interface.
var _ gpubsub.Client = &testClient{}

// Close implements client.Close
func (c *testClient) Close() error {
	return c.data.CloseErr
}

// Topic implements Client.Topic.
func (c *testClient) Topic(id string) gpubsub.Topic {
	return &testTopic{data: c.data.TopicData, handleData: c.data.HandleData, id: id}
}

// Subscription implements Client.Subscription.
func (c *testClient) Subscription(id string) gpubsub.Subscription {
	return &testSubscription{data: c.data.SubscriptionData, id: id}
}

// CreateSubscription implements Client.CreateSubscription.
func (c *testClient) CreateSubscription(ctx context.Context, id string, cfg gpubsub.SubscriptionConfig) (gpubsub.Subscription, error) {
	return &testSubscription{data: c.data.SubscriptionData, id: id}, c.data.CreateSubscriptionErr
}

// CreateTopic implements pubsub.Client.CreateTopic
func (c *testClient) CreateTopic(ctx context.Context, id string) (gpubsub.Topic, error) {
	return &testTopic{data: c.data.TopicData, handleData: c.data.HandleData, id: id}, c.data.CreateTopicErr
}

func (c *testClient) CreateTopicWithConfig(ctx context.Context, id string, cfg *pubsub.TopicConfig) (gpubsub.Topic, error) {
	return &testTopic{data: c.data.TopicData, handleData: c.data.HandleData, id: id, config: cfg}, c.data.CreateTopicErr
}
