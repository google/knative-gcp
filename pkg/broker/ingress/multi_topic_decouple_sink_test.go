/*
Copyright 2020 Google LLC

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

package ingress

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"

	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client/test"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	logtest "knative.dev/pkg/logging/testing"
)

func TestMultiTopicDecoupleSink(t *testing.T) {
	type brokerTestCase struct {
		broker  types.NamespacedName
		topic   string
		wantErr bool
	}
	tests := []struct {
		name         string
		brokerConfig *config.TargetsConfig
		cases        []brokerTestCase
	}{
		{
			name: "happy path single broker",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1", State: config.State_READY}},
				},
			},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic: "test_topic_1",
				},
			},
		},
		{
			name: "happy path multiple brokers",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1", State: config.State_READY}},
					"test_ns_2/test_broker_2": {DecoupleQueue: &config.Queue{Topic: "test_topic_2", State: config.State_READY}},
				},
			},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic: "test_topic_1",
				},
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_2",
						Name:      "test_broker_2",
					},
					topic: "test_topic_2",
				},
			},
		},
		{
			name:         "broker doesn't exist in config",
			brokerConfig: &config.TargetsConfig{},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic:   "test_topic_1",
					wantErr: true,
				},
			},
		},
		{
			name: "broker is not ready",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1", State: config.State_UNKNOWN}},
				},
			},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic:   "test_topic_1",
					wantErr: true,
				},
			},
		},
		{
			name: "decouple queue is nil for broker",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: nil},
				},
			},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic:   "test_topic_1",
					wantErr: true,
				},
			},
		},
		{
			name: "empty topic for broker",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: ""}},
				},
			},
			cases: []brokerTestCase{
				{
					broker: types.NamespacedName{
						Namespace: "test_ns_1",
						Name:      "test_broker_1",
					},
					topic:   "test_topic_1",
					wantErr: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := logtest.TestContextWithLogger(t)
			psSrv := pstest.NewServer()
			defer psSrv.Close()
			psClient := createPubsubClient(ctx, t, psSrv)
			brokerConfig := memory.NewTargets(tt.brokerConfig)
			for i, testCase := range tt.cases {
				topic := psClient.Topic(testCase.topic)
				if exists, err := topic.Exists(ctx); err != nil {
					t.Fatal(err)
				} else if !exists {
					if topic, err = psClient.CreateTopic(ctx, testCase.topic); err != nil {
						t.Fatal(err)
					}
				}
				subscription, err := psClient.CreateSubscription(
					ctx, fmt.Sprintf("test-sub-%d", i), pubsub.SubscriptionConfig{Topic: topic})
				if err != nil {
					t.Fatal(err)
				}

				sink := NewMultiTopicDecoupleSink(ctx, brokerConfig, psClient, pubsub.DefaultPublishSettings)
				// Send events
				event := createTestEvent(uuid.New().String())
				err = sink.Send(context.Background(), testCase.broker, *event)

				// Verify results.
				if testCase.wantErr && err == nil {
					t.Fatal("Want error but got nil")
				}
				if !testCase.wantErr && err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if !testCase.wantErr {
					rctx, cancel := context.WithCancel(ctx)
					msgCh := make(chan *pubsub.Message, 1)
					subscription.Receive(rctx,
						func(ctx context.Context, m *pubsub.Message) {
							select {
							case msgCh <- m:
								cancel()
							case <-ctx.Done():
							}
							m.Ack()
						},
					)
					msg := <-msgCh
					if got, err := binding.ToEvent(ctx, cepubsub.NewMessage(msg)); err != nil {
						t.Error(err)
					} else if diff := cmp.Diff(event, got); diff != "" {
						t.Errorf("Output event doesn't match input, diff: %v", diff)
					}
				}
			}
		})
	}
}

type fakePubsubClient struct {
	t *testing.T
	// topics is the mapping from topic name to corresponding channel which contains the event.
	topics        map[string]<-chan cloudevents.Event
	topicToClient map[string]cloudevents.Client
	// topicToErr injects error to a topic to simulate client error.
	topicToErr map[string]error
}

func newFakePubsubClient(t *testing.T) *fakePubsubClient {
	return &fakePubsubClient{
		t:             t,
		topics:        map[string]<-chan cloudevents.Event{},
		topicToClient: map[string]cloudevents.Client{},
		topicToErr:    map[string]error{},
	}
}

func (c *fakePubsubClient) Send(ctx context.Context, event cloudevents.Event) protocol.Result {
	topic := cecontext.TopicFrom(ctx)
	if err := c.topicToErr[topic]; err != nil {
		return err
	}
	_, ok := c.topicToClient[topic]
	if !ok {
		c.topicToClient[topic], c.topics[topic] = test.NewMockSenderClient(c.t, 1)
	}
	return c.topicToClient[topic].Send(ctx, event)
}

func (c *fakePubsubClient) Request(ctx context.Context, event event.Event) (*event.Event, protocol.Result) {
	// noop
	return nil, nil
}

func (c *fakePubsubClient) StartReceiver(ctx context.Context, fn interface{}) error {
	// noop
	return nil
}

func injectErr(topic string, err error) func(client *fakePubsubClient) {
	return func(client *fakePubsubClient) {
		client.topicToErr[topic] = err
	}
}
