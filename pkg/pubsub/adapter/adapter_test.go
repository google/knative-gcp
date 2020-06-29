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

package adapter

import (
	"context"
	"errors"

	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/utils/clients"
	logtest "knative.dev/pkg/logging/testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/cloudevents/sdk-go/v2/event"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const (
	testProjectID     = "test-testProjectID"
	testTopic         = "test-testTopic"
	testSub           = "test-testSub"
	testName          = "test-testName"
	testNamespace     = "test-testNamespace"
	testResourceGroup = "test-testResourceGroup"
	testConverterType = "test-testConverterType"
)

func testPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pubsub.Client, func()) {
	t.Helper()
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial test pubsub connection: %v", err)
	}
	close := func() {
		srv.Close()
		conn.Close()
	}
	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("failed to create test pubsub client: %v", err)
	}
	return c, close
}

type metricLabels struct {
	CeType     string
	CeSource   string
	StatusCode int
}

type statsReporterRecorder struct {
	labels []metricLabels
}

func (r *statsReporterRecorder) ReportEventCount(args *ReportArgs, responseCode int) error {
	r.labels = append(r.labels, metricLabels{CeType: args.EventType, CeSource: args.EventSource, StatusCode: responseCode})
	return nil
}

type mockConverter struct {
	converted *cev2.Event
}

func (c *mockConverter) Convert(ctx context.Context, msg *pubsub.Message, converterType converters.ConverterType) (*cev2.Event, error) {
	if c.converted == nil {
		return nil, errors.New("induced error")
	}
	return c.converted, nil
}

func TestAdapter(t *testing.T) {
	sampleEvent := newSampleEvent()
	convertedEvent := sampleEvent.Clone()
	convertedEvent.SetID("converted")
	replyEvent := convertedEvent.Clone()
	replyEvent.SetType("new-type")

	cases := []struct {
		name             string
		original         *event.Event
		converted        *event.Event
		reply            *event.Event
		wantMetricLabels []metricLabels
	}{{
		name:     "converter fails",
		original: sampleEvent,
	}, {
		name:      "successful with no reply",
		original:  sampleEvent,
		converted: &convertedEvent,
		wantMetricLabels: []metricLabels{{
			CeType:     convertedEvent.Type(),
			CeSource:   convertedEvent.Source(),
			StatusCode: http.StatusOK,
		}},
	}, {
		name:      "successful with reply",
		original:  sampleEvent,
		converted: &convertedEvent,
		reply:     &replyEvent,
		wantMetricLabels: []metricLabels{{
			CeType:     convertedEvent.Type(),
			CeSource:   convertedEvent.Source(),
			StatusCode: http.StatusOK,
		}, {
			CeType:     replyEvent.Type(),
			CeSource:   replyEvent.Source(),
			StatusCode: http.StatusOK,
		}},
	}}

	// TODO add reply failures and other cases

	for _, tc := range cases {
		// Shadowing the loop iteration variable inside the loop to avoid a race otherwise.
		// TODO find a better fix
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := logtest.TestContextWithLogger(t)

			transformerClient, err := cehttp.New()
			if err != nil {
				t.Fatalf("failed to create transformer cloudevents client: %v", err)
			}
			transformerSvr := httptest.NewServer(transformerClient)
			defer transformerSvr.Close()

			sinkClient, err := cehttp.New()
			if err != nil {
				t.Fatalf("failed to create sink cloudevents client: %v", err)
			}
			sinkSvr := httptest.NewServer(sinkClient)
			defer sinkSvr.Close()

			c, close := testPubsubClient(ctx, t, testProjectID)
			defer close()

			topic, err := c.CreateTopic(ctx, testTopic)
			if err != nil {
				t.Fatalf("failed to create topic: %v", err)
			}
			sub, err := c.CreateSubscription(ctx, testSub, pubsub.SubscriptionConfig{
				Topic: topic,
			})
			if err != nil {
				t.Fatalf("failed to create subscription: %v", err)
			}

			p, err := cepubsub.New(context.Background(),
				cepubsub.WithClient(c),
				cepubsub.WithProjectID(testProjectID),
				cepubsub.WithTopicID(testTopic),
			)
			if err != nil {
				t.Fatalf("failed to create cloudevents pubsub protocol: %v", err)
			}

			outbound := http.DefaultClient

			args := &AdapterArgs{
				TopicID:       testTopic,
				SinkURI:       sinkSvr.URL,
				Extensions:    map[string]string{},
				ConverterType: converters.ConverterType(testConverterType),
			}

			if tc.reply != nil {
				args.TransformerURI = transformerSvr.URL
			}

			adapter := NewAdapter(ctx,
				clients.ProjectID(testProjectID),
				Namespace(testNamespace),
				Name(testName),
				ResourceGroup(testResourceGroup),
				sub,
				outbound,
				&mockConverter{converted: tc.converted},
				&statsReporterRecorder{},
				args)

			errCh := make(chan error, 1)
			go func() {
				errCh <- adapter.Start(ctx)
			}()
			defer adapter.Stop()

			select {
			case err := <-errCh:
				t.Fatalf("Failed to start adapter: %v", err)
			// TODO better way of doing this?
			case <-time.After(time.Second):
				t.Logf("Adapter started")
			}

			rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			go func() {
				msg, resp, err := transformerClient.Respond(rctx)
				if err != nil && err != io.EOF {
					t.Errorf("unexpected error from transformer receiving event: %v", err)
				}
				if msg != nil {
					defer msg.Finish(nil)

					gotEvent, err := binding.ToEvent(rctx, msg)
					if err != nil {
						t.Errorf("transformer received message cannot be converted to an event: %v", err)
					}
					if diff := cmp.Diff(tc.converted, gotEvent); diff != "" {
						t.Errorf("transformer received event (-want,+got): %v", diff)
					}

					if tc.reply != nil {
						if err := resp(rctx, binding.ToMessage(tc.reply), protocol.ResultACK); err != nil {
							t.Errorf("unexpected error from transfomer responding event: %v", err)
						}
					}
				}
			}()

			go func() {
				msg, err := sinkClient.Receive(rctx)
				if err != nil && err != io.EOF {
					t.Errorf("unexpected error from sink when receiving event: %v", err)
				}
				var gotEvent *event.Event
				if msg != nil {
					defer msg.Finish(nil)
					var err error
					gotEvent, err = binding.ToEvent(rctx, msg)
					if err != nil {
						t.Errorf("sink received message that cannot be converted to an event: %v", err)
					}
				}
				wantEvent := tc.converted
				if tc.reply != nil {
					wantEvent = tc.reply
				}
				if diff := cmp.Diff(wantEvent, gotEvent); diff != "" {
					t.Errorf("sink received event (-want,+got): %v", diff)
				}
			}()

			if err := p.Send(ctx, binding.ToMessage(tc.original)); err != nil {
				t.Fatalf("failed to seed event to pubsub: %v", err)
			}

			<-rctx.Done()

			gotMetricLabels := adapter.reporter.(*statsReporterRecorder).labels
			if diff := cmp.Diff(tc.wantMetricLabels, gotMetricLabels); diff != "" {
				t.Errorf("metrics reported (-want,+got): %v", diff)
			}

		})
	}
}

func newSampleEvent() *event.Event {
	sampleEvent := event.New()
	sampleEvent.SetID("id")
	sampleEvent.SetSource("source")
	sampleEvent.SetSubject("subject")
	sampleEvent.SetType("type")
	sampleEvent.SetTime(time.Now())
	return &sampleEvent
}
