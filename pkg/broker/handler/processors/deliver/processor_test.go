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

package deliver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/cloudevents/sdk-go/v2/binding"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/broker/eventutil"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
)

func TestInvalidContext(t *testing.T) {
	p := &Processor{Targets: memory.NewEmptyTargets()}
	e := event.New()
	err := p.Process(context.Background(), &e)
	if err != handlerctx.ErrBrokerKeyNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrBrokerKeyNotPresent)
	}

	ctx := handlerctx.WithBrokerKey(context.Background(), "key")
	err = p.Process(ctx, &e)
	if err != handlerctx.ErrTargetKeyNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrTargetKeyNotPresent)
	}
}

func TestDeliverSuccess(t *testing.T) {
	targetClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create target cloudevents client: %v", err)
	}
	ingressClient, err := cehttp.New()
	if err != nil {
		t.Fatalf("failed to create ingress cloudevents client: %v", err)
	}
	deliverClient, err := ceclient.NewDefault()
	if err != nil {
		t.Fatalf("failed to create requester cloudevents client: %v", err)
	}
	targetSvr := httptest.NewServer(targetClient)
	defer targetSvr.Close()
	ingressSvr := httptest.NewServer(ingressClient)
	defer ingressSvr.Close()

	broker := &config.Broker{Namespace: "ns", Name: "broker"}
	target := &config.Target{Namespace: "ns", Name: "target", Broker: "broker", Address: targetSvr.URL}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
		bm.SetAddress(ingressSvr.URL)
		bm.UpsertTargets(target)
	})
	ctx := handlerctx.WithBrokerKey(context.Background(), broker.Key())
	ctx = handlerctx.WithTargetKey(ctx, target.Key())

	p := &Processor{
		DeliverClient: deliverClient,
		Targets:       testTargets,
		EventTTL:      &eventutil.TTL{Logger: zap.NewNop()},
	}

	origin := event.New()
	origin.SetID("id")
	origin.SetSource("source")
	origin.SetSubject("subject")
	origin.SetType("type")
	origin.SetTime(time.Now())

	reply := origin.Clone()
	reply.SetID("reply")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, resp, err := targetClient.Respond(ctx)
		if err != nil {
			t.Errorf("unexpected error from target receiving event: %v", err)
		}
		if err := resp(ctx, binding.ToMessage(&reply), protocol.ResultACK); err != nil {
			t.Errorf("unexpected error from target responding event: %v", err)
		}
		defer msg.Finish(nil)
		gotEvent, err := binding.ToEvent(ctx, msg)
		if err != nil {
			t.Errorf("target received message cannot be converted to an event: %v", err)
		}
		// Force the time to be the same so that we can compare easier.
		gotEvent.SetTime(origin.Time())
		if diff := cmp.Diff(&origin, gotEvent); diff != "" {
			t.Errorf("target received event (-want,+got): %v", diff)
		}
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg, err := ingressClient.Receive(ctx)
		if err != nil {
			t.Errorf("unexpected error from ingress receiving event: %v", err)
		}
		defer msg.Finish(nil)
		gotEvent, err := binding.ToEvent(ctx, msg)
		if err != nil {
			t.Errorf("ingress received message cannot be converted to an event: %v", err)
		}
		// Force the time to be the same so that we can compare easier.
		if diff := cmp.Diff(&reply, gotEvent); diff != "" {
			t.Errorf("ingress received event (-want,+got): %v", diff)
		}
	}()

	if err := p.Process(ctx, &origin); err != nil {
		t.Errorf("unexpected error from processing: %v", err)
	}
}

func TestDeliverFailure(t *testing.T) {
	cases := []struct {
		name          string
		withRetry     bool
		withRespDelay time.Duration
		failRetry     bool
		wantErr       bool
	}{{
		name:    "delivery error no retry",
		wantErr: true,
	}, {
		name:      "delivery error retry success",
		withRetry: true,
	}, {
		name:      "delivery error retry failure",
		withRetry: true,
		failRetry: true,
		wantErr:   true,
	}, {
		name:          "delivery timeout no retry",
		withRespDelay: time.Second,
		wantErr:       true,
	}, {
		name:          "delivery timeout retry success",
		withRespDelay: time.Second,
		withRetry:     true,
	}, {
		name:          "delivery timeout retry failure",
		withRetry:     true,
		withRespDelay: time.Second,
		failRetry:     true,
		wantErr:       true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			targetClient, err := cehttp.New()
			if err != nil {
				t.Fatalf("failed to create target cloudevents client: %v", err)
			}
			deliverClient, err := ceclient.NewDefault()
			if err != nil {
				t.Fatalf("failed to create requester cloudevents client: %v", err)
			}
			targetSvr := httptest.NewServer(targetClient)
			defer targetSvr.Close()

			_, c, close := testPubsubClient(ctx, t, "test-project")
			defer close()

			// Don't create the retry topic to make it fail.
			if !tc.failRetry {
				if _, err := c.CreateTopic(ctx, "test-retry-topic"); err != nil {
					t.Fatalf("failed to create test pubsub topc: %v", err)
				}
			}

			ps, err := cepubsub.New(ctx, cepubsub.WithClient(c), cepubsub.WithProjectID("test-project"))
			if err != nil {
				t.Fatalf("failed to create pubsub protocol: %v", err)
			}
			deliverRetryClient, err := ceclient.New(ps)
			if err != nil {
				t.Fatalf("failed to create cloudevents client: %v", err)
			}

			broker := &config.Broker{Namespace: "ns", Name: "broker"}
			target := &config.Target{
				Namespace: "ns",
				Name:      "target",
				Broker:    "broker",
				Address:   targetSvr.URL,
				RetryQueue: &config.Queue{
					Topic: "test-retry-topic",
				},
			}
			testTargets := memory.NewEmptyTargets()
			testTargets.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
				bm.UpsertTargets(target)
			})
			ctx = handlerctx.WithBrokerKey(ctx, broker.Key())
			ctx = handlerctx.WithTargetKey(ctx, target.Key())

			p := &Processor{
				DeliverClient:      deliverClient,
				Targets:            testTargets,
				RetryOnFailure:     tc.withRetry,
				DeliverRetryClient: deliverRetryClient,
				DeliverTimeout:     500 * time.Millisecond,
			}

			origin := event.New()
			origin.SetID("id")
			origin.SetSource("source")
			origin.SetSubject("subject")
			origin.SetType("type")
			origin.SetTime(time.Now())

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				msg, resp, err := targetClient.Respond(ctx)
				if err != nil {
					t.Errorf("unexpected error from target receiving event: %v", err)
				}
				defer msg.Finish(nil)

				// If with delay, we reply OK so that we know the error is for sure caused by timeout.
				if tc.withRespDelay > 0 {
					time.Sleep(tc.withRespDelay)
					if err := resp(ctx, nil, &cehttp.Result{StatusCode: http.StatusOK}); err != nil {
						t.Errorf("unexpected error from target responding event: %v", err)
					}
					return
				}

				// Due to https://github.com/cloudevents/sdk-go/issues/433
				// it's not possible to use Receive to easily return error.
				if err := resp(ctx, nil, &cehttp.Result{StatusCode: http.StatusInternalServerError}); err != nil {
					t.Errorf("unexpected error from target responding event: %v", err)
				}
			}()

			err = p.Process(ctx, &origin)
			if (err != nil) != tc.wantErr {
				t.Errorf("processing got error=%v, want=%v", err, tc.wantErr)
			}
		})
	}
}

func toFakePubsubMessage(m *pstest.Message) *pubsub.Message {
	return &pubsub.Message{
		ID:         m.ID,
		Attributes: m.Attributes,
		Data:       m.Data,
	}
}

func testPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pstest.Server, *pubsub.Client, func()) {
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
	return srv, c, close
}
