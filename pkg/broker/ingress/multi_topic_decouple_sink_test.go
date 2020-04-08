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
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/client/test"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
)

func TestMultiTopicDecoupleSink(t *testing.T) {
	type brokerTestCase struct {
		ns          string
		broker      string
		topic       string
		clientErrFn func(client *fakePubsubClient)
		wantErr     bool
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
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1"}},
				},
			},
			cases: []brokerTestCase{
				{
					ns:     "test_ns_1",
					broker: "test_broker_1",
					topic:  "test_topic_1",
				},
			},
		},
		{
			name: "happy path multiple brokers",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1"}},
					"test_ns_2/test_broker_2": {DecoupleQueue: &config.Queue{Topic: "test_topic_2"}},
				},
			},
			cases: []brokerTestCase{
				{
					ns:     "test_ns_1",
					broker: "test_broker_1",
					topic:  "test_topic_1",
				},
				{
					ns:     "test_ns_2",
					broker: "test_broker_2",
					topic:  "test_topic_2",
				},
			},
		},
		{
			name: "client returns error",
			brokerConfig: &config.TargetsConfig{
				Brokers: map[string]*config.Broker{
					"test_ns_1/test_broker_1": {DecoupleQueue: &config.Queue{Topic: "test_topic_1"}},
				},
			},
			cases: []brokerTestCase{
				{
					ns:          "test_ns_1",
					broker:      "test_broker_1",
					topic:       "test_topic_1",
					clientErrFn: injectErr("test_topic_1", errors.New("inject error")),
					wantErr:     true,
				},
			},
		},
		{
			name:         "broker doesn't exist in config",
			brokerConfig: &config.TargetsConfig{},
			cases: []brokerTestCase{
				{
					ns:      "test_ns_1",
					broker:  "test_broker_1",
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
					ns:      "test_ns_1",
					broker:  "test_broker_1",
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
					ns:      "test_ns_1",
					broker:  "test_broker_1",
					topic:   "test_topic_1",
					wantErr: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newFakePubsubClient(t)
			brokerConfig := memory.NewTargets(tt.brokerConfig)
			for _, testCase := range tt.cases {
				if testCase.clientErrFn != nil {
					testCase.clientErrFn(fakeClient)
				}
				sink, err := NewMultiTopicDecoupleSink(context.Background(), WithBrokerConfig(brokerConfig), WithPubsubClient(fakeClient))
				if err != nil {
					t.Fatalf("Failed to create decouple sink: %v", err)
				}

				// Send events
				event := createTestEvent(uuid.New().String())
				err = sink.Send(context.Background(), testCase.ns, testCase.broker, *event)

				// Verify results.
				if testCase.wantErr && err == nil {
					t.Fatal("Want error but got nil")
				}
				if !testCase.wantErr && err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if !testCase.wantErr {
					got := <-fakeClient.topics[testCase.topic]
					if dif := cmp.Diff(*event, got); dif != "" {
						t.Errorf("Output event doesn't match input, dif: %v", dif)
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
