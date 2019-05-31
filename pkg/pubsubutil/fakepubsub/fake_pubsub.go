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

package fakepubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsubutil"
	"golang.org/x/oauth2/google"
)

type CreatorData struct {
	ClientCreateErr error
	ClientData      ClientData
}

func Creator(value interface{}) pubsubutil.PubSubClientCreator {
	var data CreatorData
	var ok bool
	if data, ok = value.(CreatorData); !ok {
		data = CreatorData{}
	}
	if data.ClientCreateErr != nil {
		return func(_ context.Context, _ *google.Credentials, _ string) (pubsubutil.PubSubClient, error) {
			return nil, data.ClientCreateErr
		}
	}
	return func(_ context.Context, credentials *google.Credentials, _ string) (pubsubutil.PubSubClient, error) {
		return &Client{
			Data: data.ClientData,
		}, nil
	}
}

type ClientData struct {
	SubscriptionData SubscriptionData
	CreateSubErr     error
	TopicData        TopicData
	CreateTopicErr   error
}

type Client struct {
	Data ClientData
}

var _ pubsubutil.PubSubClient = &Client{}

func (c *Client) SubscriptionInProject(id, projectId string) pubsubutil.PubSubSubscription {
	return &Subscription{
		Data: c.Data.SubscriptionData,
	}
}

func (c *Client) CreateSubscription(ctx context.Context, id string, topic pubsubutil.PubSubTopic) (pubsubutil.PubSubSubscription, error) {
	if c.Data.CreateSubErr != nil {
		return nil, c.Data.CreateSubErr
	}
	return &Subscription{
		Data: c.Data.SubscriptionData,
	}, nil
}

func (c *Client) Topic(id string) pubsubutil.PubSubTopic {
	return &Topic{
		Data: c.Data.TopicData,
	}
}

func (c *Client) CreateTopic(ctx context.Context, id string) (pubsubutil.PubSubTopic, error) {
	if c.Data.CreateTopicErr != nil {
		return nil, c.Data.CreateTopicErr
	}
	return c.Topic(id), nil
}

type SubscriptionData struct {
	Exists     bool
	ExistsErr  error
	DeleteErr  error
	ReceiveErr error

	ReceiveFunc func(context.Context, pubsubutil.PubSubMessage)
}

type Subscription struct {
	Data SubscriptionData
}

var _ pubsubutil.PubSubSubscription = &Subscription{}

func (s *Subscription) Exists(ctx context.Context) (bool, error) {
	return s.Data.Exists, s.Data.ExistsErr
}

func (s *Subscription) ID() string {
	return "test-subscription-id"
}

func (s *Subscription) Delete(ctx context.Context) error {
	return s.Data.DeleteErr
}

func (s *Subscription) Receive(ctx context.Context, f func(context.Context, pubsubutil.PubSubMessage)) error {
	s.Data.ReceiveFunc = f
	return s.Data.ReceiveErr
}

type TopicData struct {
	Exists    bool
	ExistsErr error

	DeleteErr error
	Publish   PublishResultData

	Stop bool
}

type Topic struct {
	Data TopicData
}

var _ pubsubutil.PubSubTopic = &Topic{}

func (t *Topic) Exists(ctx context.Context) (bool, error) {
	return t.Data.Exists, t.Data.ExistsErr
}

func (t *Topic) ID() string {
	return "test-topic-ID"
}

func (t *Topic) Delete(ctx context.Context) error {
	return t.Data.DeleteErr
}

func (t *Topic) Publish(ctx context.Context, msg *pubsub.Message) pubsubutil.PubSubPublishResult {
	return &PublishResult{
		Data: t.Data.Publish,
	}
}

func (t *Topic) Stop() {
	t.Data.Stop = true
}

type PublishResultData struct {
	ID    string
	Err   error
	Ready <-chan struct{}
}

type PublishResult struct {
	Data PublishResultData
}

var _ pubsubutil.PubSubPublishResult = &PublishResult{}

func (r *PublishResult) Ready() <-chan struct{} {
	return r.Data.Ready
}

func (r *PublishResult) Get(ctx context.Context) (serverID string, err error) {
	return r.Data.ID, r.Data.Err
}

type MessageData struct {
	Ack  bool
	Nack bool
}

type Message struct {
	MessageData MessageData
}

var _ pubsubutil.PubSubMessage = &Message{}

func (m *Message) ID() string {
	return "test-message-id"
}

func (m *Message) Data() []byte {
	return []byte("test-message-data")
}

func (m *Message) Attributes() map[string]string {
	return map[string]string{
		"test": "attributes",
	}
}

func (m *Message) Ack() {
	m.MessageData.Ack = true
}

func (m *Message) Nack() {
	m.MessageData.Nack = true
}
