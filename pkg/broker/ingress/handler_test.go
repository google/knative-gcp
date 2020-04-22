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
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"knative.dev/pkg/logging"
	logtest "knative.dev/pkg/logging/testing"
)

const (
	projectID      = "testproject"
	topicID        = "topic1"
	subscriptionID = "subscription1"
)

var brokerConfig = &config.TargetsConfig{
	Brokers: map[string]*config.Broker{
		"ns1/broker1": {
			Id:            "b-uid-1",
			Name:          "broker1",
			Namespace:     "ns1",
			DecoupleQueue: &config.Queue{Topic: topicID},
		},
		"ns2/broker2": {
			Id:            "b-uid-2",
			Name:          "broker2",
			Namespace:     "ns2",
			DecoupleQueue: nil,
		},
		"ns3/broker3": {
			Id:            "b-uid-3",
			Name:          "broker3",
			Namespace:     "ns3",
			DecoupleQueue: &config.Queue{Topic: ""},
		},
	},
}

type testCase struct {
	name string
	// in happy case, path should match the /<ns>/<broker> in the brokerConfig.
	path  string
	event *cloudevents.Event
	// If method is empty, POST will be used as default.
	method string
	// body and header can be specified if the client is making raw HTTP request instead of via cloudevents.
	body     map[string]string
	header   nethttp.Header
	wantCode int
}

func TestHandler(t *testing.T) {
	tests := []testCase{
		{
			name:     "happy case",
			path:     "/ns1/broker1",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusOK,
		},
		{
			name:     "valid event but unsupported http  method",
			method:   "PUT",
			path:     "/ns1/broker1",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusMethodNotAllowed,
		},
		{
			name:     "malformed path",
			path:     "/ns1/broker1/and/something/else",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusNotFound,
		},
		{
			name:     "request is not an event",
			path:     "/ns1/broker1",
			wantCode: nethttp.StatusBadRequest,
			header:   nethttp.Header{},
		},
		{
			name:     "wrong path - broker doesn't exist in given namespace",
			path:     "/ns1/broker-not-exist",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusNotFound,
		},
		{
			name:     "wrong path - namespace doesn't exist",
			path:     "/ns-not-exist/broker1",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusNotFound,
		},
		{
			name:     "broker queue is nil",
			path:     "/ns2/broker2",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusInternalServerError,
		},
		{
			name:     "broker queue topic is empty",
			path:     "/ns3/broker3",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusInternalServerError,
		},
	}

	client := nethttp.Client{}
	t.Cleanup(client.CloseIdleConnections)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := logging.WithLogger(context.Background(), logtest.TestLogger(t))
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			t.Cleanup(cancel)

			psSrv := pstest.NewServer()
			t.Cleanup(func() {
				psSrv.Close()
			})

			url := createAndStartIngress(ctx, t, psSrv)
			rec := setupTestReceiver(ctx, t, psSrv)

			res, err := client.Do(createRequest(tc, url))
			if err != nil {
				t.Fatalf("Unexpected error from http client: %v", err)
			}
			if res.StatusCode != tc.wantCode {
				t.Errorf("StatusCode mismatch. got: %v, want: %v", res.StatusCode, tc.wantCode)
			}

			// If event is accepted, check that it's stored in the decouple sink.
			if res.StatusCode == nethttp.StatusOK {
				m, err := rec.Receive(ctx)
				if err != nil {
					t.Fatal(err)
				}
				savedToSink, err := binding.ToEvent(ctx, m)
				if err != nil {
					t.Fatal(err)
				}
				// Retrieve the event from the decouple sink.
				if tc.event.ID() != savedToSink.ID() {
					t.Errorf("Event ID mismatch. got: %v, want: %v", savedToSink.ID(), tc.event.ID())
				}
				if savedToSink.Time().IsZero() {
					t.Errorf("Saved event should be decorated with timestamp, got zero.")
				}
			}

			select {
			case <-ctx.Done():
				t.Fatalf("test cancelled or timed out: %v", ctx.Err())
			default:
			}
		})
	}
}

func createPubsubClient(ctx context.Context, t *testing.T, psSrv *pstest.Server) *pubsub.Client {
	conn, err := grpc.Dial(psSrv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	psClient, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	return psClient
}

func setupTestReceiver(ctx context.Context, t *testing.T, psSrv *pstest.Server) *cepubsub.Protocol {
	ps := createPubsubClient(ctx, t, psSrv)
	topic, err := ps.CreateTopic(ctx, topicID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ps.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		t.Fatal(err)
	}
	p, err := cepubsub.New(ctx, cepubsub.WithClient(ps), cepubsub.WithSubscriptionAndTopicID(subscriptionID, topicID))
	if err != nil {
		t.Fatal(err)
	}

	go p.OpenInbound(cecontext.WithLogger(ctx, logtest.TestLogger(t)))
	return p
}

// createAndStartIngress creates an ingress and calls its Start() method in a goroutine.
func createAndStartIngress(ctx context.Context, t *testing.T, psSrv *pstest.Server) string {
	decouple, err1 := NewMultiTopicDecoupleSink(ctx,
		WithBrokerConfig(memory.NewTargets(brokerConfig)),
		WithPubsubClient(createPubsubClient(ctx, t, psSrv)))
	if err1 != nil {
		t.Fatalf("Failed to create decouple sink: %v", err1)
	}

	receiver := &testHttpMessageReceiver{urlCh: make(chan string)}
	h := &handler{
		logger:       logging.FromContext(ctx).Desugar(),
		httpReceiver: receiver,
		decouple:     decouple,
	}

	errCh := make(chan error)
	go func() {
		errCh <- h.Start(ctx)
	}()
	select {
	case err := <-errCh:
		t.Fatalf("Failed to start ingress: %v", err)
	case url := <-h.httpReceiver.(*testHttpMessageReceiver).urlCh:
		return url
	}
	return ""
}

func createTestEvent(id string) *cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(id)
	event.SetSource("test-source")
	event.SetType("test-type")
	return &event
}

// createRequest creates an http request from the test case. If event is specified, it converts the event to a request.
func createRequest(tc testCase, url string) *nethttp.Request {
	method := "POST"
	if tc.method != "" {
		method = tc.method
	}
	body, _ := json.Marshal(tc.body)
	request, _ := nethttp.NewRequest(method, url+tc.path, bytes.NewBuffer(body))
	if tc.header != nil {
		request.Header = tc.header
	}
	if tc.event != nil {
		message := binding.ToMessage(tc.event)
		defer message.Finish(nil)
		http.WriteRequest(context.Background(), message, request)
	}
	return request
}

// testHttpMessageReceiver implements HttpMessageReceiver. When created, it creates an httptest.Server,
// which starts a server with any available port.
type testHttpMessageReceiver struct {
	// once the server is started, the server's url is sent to this channel.
	urlCh chan string
}

func (recv *testHttpMessageReceiver) StartListen(ctx context.Context, handler nethttp.Handler) error {
	// NewServer creates a new server and starts it. It is non-blocking.
	server := httptest.NewServer(handler)
	defer server.Close()
	recv.urlCh <- server.URL
	select {
	case <-ctx.Done():
		server.Close()
	}
	return nil
}
