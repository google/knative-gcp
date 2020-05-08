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
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/metrics"
	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
	logtest "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

const (
	projectID      = "testproject"
	topicID        = "topic1"
	subscriptionID = "subscription1"

	traceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	eventType = "test-event-type"
	pod       = "testpod"
	container = "testcontainer"
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

type eventAssertion func(*testing.T, *cloudevents.Event)

type testCase struct {
	name string
	// in happy case, path should match the /<ns>/<broker> in the brokerConfig.
	path  string
	event *cloudevents.Event
	// If method is empty, POST will be used as default.
	method string
	// body and header can be specified if the client is making raw HTTP request instead of via cloudevents.
	body           map[string]string
	header         nethttp.Header
	wantCode       int
	wantMetricTags map[string]string
	wantEventCount int64
	// additional assertions on the output event.
	eventAssertions []eventAssertion
}

func TestHandler(t *testing.T) {
	tests := []testCase{
		{
			name:           "happy case",
			path:           "/ns1/broker1",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusAccepted,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns1",
				metricskey.LabelBrokerName:        "broker1",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "202",
				metricskey.LabelResponseCodeClass: "2xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
			eventAssertions: []eventAssertion{assertExtensionsExist(EventArrivalTime)},
		},
		{
			name:     "trace context",
			path:     "/ns1/broker1",
			event:    createTestEvent("test-event"),
			wantCode: nethttp.StatusAccepted,
			header: nethttp.Header{
				"Traceparent": {fmt.Sprintf("00-%s-00f067aa0ba902b7-01", traceID)},
			},
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns1",
				metricskey.LabelBrokerName:        "broker1",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "202",
				metricskey.LabelResponseCodeClass: "2xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
			eventAssertions: []eventAssertion{assertExtensionsExist(EventArrivalTime), assertTraceID(traceID)},
		},
		{
			name:     "valid event but unsupported http method",
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
			name:           "wrong path - broker doesn't exist in given namespace",
			path:           "/ns1/broker-not-exist",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusNotFound,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns1",
				metricskey.LabelBrokerName:        "broker-not-exist",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "404",
				metricskey.LabelResponseCodeClass: "4xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
		{
			name:           "wrong path - namespace doesn't exist",
			path:           "/ns-not-exist/broker1",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusNotFound,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns-not-exist",
				metricskey.LabelBrokerName:        "broker1",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "404",
				metricskey.LabelResponseCodeClass: "4xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
		{
			name:           "broker queue is nil",
			path:           "/ns2/broker2",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusInternalServerError,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns2",
				metricskey.LabelBrokerName:        "broker2",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "500",
				metricskey.LabelResponseCodeClass: "5xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
		{
			name:           "broker queue topic is empty",
			path:           "/ns3/broker3",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusInternalServerError,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelNamespaceName:     "ns3",
				metricskey.LabelBrokerName:        "broker3",
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "500",
				metricskey.LabelResponseCodeClass: "5xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
	}

	client := nethttp.Client{}
	defer client.CloseIdleConnections()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reportertest.ResetIngressMetrics()
			ctx := logging.WithLogger(context.Background(), logtest.TestLogger(t))
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			psSrv := pstest.NewServer()
			defer psSrv.Close()

			url, cancel1 := createAndStartIngress(ctx, t, psSrv)
			defer cancel1()
			rec, cancel2 := setupTestReceiver(ctx, t, psSrv)
			defer cancel2()

			res, err := client.Do(createRequest(tc, url))
			if err != nil {
				t.Fatalf("Unexpected error from http client: %v", err)
			}
			if res.StatusCode != tc.wantCode {
				t.Errorf("StatusCode mismatch. got: %v, want: %v", res.StatusCode, tc.wantCode)
			}
			verifyMetrics(t, tc)

			// If event is accepted, check that it's stored in the decouple sink.
			if res.StatusCode == nethttp.StatusAccepted {
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
				for _, assertion := range tc.eventAssertions {
					assertion(t, savedToSink)
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

func createPubsubClient(ctx context.Context, t *testing.T, psSrv *pstest.Server) (*pubsub.Client, func()) {
	conn, err := grpc.Dial(psSrv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	cancel := func() { conn.Close() }

	psClient, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	return psClient, cancel
}

func setupTestReceiver(ctx context.Context, t *testing.T, psSrv *pstest.Server) (*cepubsub.Protocol, func()) {
	ps, cancel := createPubsubClient(ctx, t, psSrv)
	topic, err := ps.CreateTopic(ctx, topicID)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	_, err = ps.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	p, err := cepubsub.New(ctx, cepubsub.WithClient(ps), cepubsub.WithSubscriptionAndTopicID(subscriptionID, topicID))
	if err != nil {
		cancel()
		t.Fatal(err)
	}

	go p.OpenInbound(cecontext.WithLogger(ctx, logtest.TestLogger(t)))
	return p, cancel
}

// createAndStartIngress creates an ingress and calls its Start() method in a goroutine.
func createAndStartIngress(ctx context.Context, t *testing.T, psSrv *pstest.Server) (string, func()) {
	p, cancel := createPubsubClient(ctx, t, psSrv)
	client, err := NewPubsubDecoupleClient(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	decouple := NewMultiTopicDecoupleSink(ctx, memory.NewTargets(brokerConfig), client)

	receiver := &testHttpMessageReceiver{urlCh: make(chan string)}
	statsReporter, err := metrics.NewIngressReporter(metrics.PodName(pod), metrics.ContainerName(container))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	h := NewHandler(ctx, receiver, decouple, statsReporter)

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Start(ctx)
	}()
	select {
	case err := <-errCh:
		cancel()
		t.Fatalf("Failed to start ingress: %v", err)
	case url := <-h.httpReceiver.(*testHttpMessageReceiver).urlCh:
		return url, cancel
	}
	return "", cancel
}

func createTestEvent(id string) *cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(id)
	event.SetSource("test-source")
	event.SetType(eventType)
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

// verifyMetrics verifies broker metrics are properly recorded (or not recorded)
func verifyMetrics(t *testing.T, tc testCase) {
	if tc.wantEventCount == 0 {
		metricstest.CheckStatsNotReported(t, "event_count", "event_dispatch_latencies")
	} else {
		metricstest.CheckStatsReported(t, "event_count", "event_dispatch_latencies")
		metricstest.CheckCountData(t, "event_count", tc.wantMetricTags, tc.wantEventCount)
		metricstest.CheckDistributionCount(t, "event_dispatch_latencies", tc.wantMetricTags, tc.wantEventCount)
	}
}

func assertTraceID(id string) eventAssertion {
	return func(t *testing.T, e *cloudevents.Event) {
		dt, ok := extensions.GetDistributedTracingExtension(*e)
		if !ok {
			t.Errorf("event missing distributed tracing extensions: %v", e)
			return
		}
		sc, err := dt.ToSpanContext()
		if err != nil {
			t.Error(err)
			return
		}
		if sc.TraceID.String() != id {
			t.Errorf("unexpected trace ID: got %q, want %q", sc.TraceID, id)
		}
	}
}

func assertExtensionsExist(extensions ...string) eventAssertion {
	return func(t *testing.T, e *cloudevents.Event) {
		for _, extension := range extensions {
			if _, ok := e.Extensions()[extension]; !ok {
				t.Errorf("Extension %v doesn't exist.", extension)
			}
		}
	}
}

// testHttpMessageReceiver implements HttpMessageReceiver. When created, it creates an httptest.Server,
// which starts a server with any available port.
type testHttpMessageReceiver struct {
	// once the server is started, the server's url is sent to this channel.
	urlCh chan string
}

func (recv *testHttpMessageReceiver) StartListen(ctx context.Context, handler nethttp.Handler) error {
	// NewServer creates a new server and starts it. It is non-blocking.
	server := httptest.NewServer(kncloudevents.CreateHandler(handler))
	defer server.Close()
	recv.urlCh <- server.URL
	select {
	case <-ctx.Done():
		server.Close()
	}
	return nil
}
