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
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cev2 "github.com/cloudevents/sdk-go/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/metrics"
	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"
	kgcptesting "github.com/google/knative-gcp/pkg/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/api/option"
	"google.golang.org/api/support/bundler"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
	logtest "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"

	_ "knative.dev/pkg/metrics/testing"
)

const (
	projectID      = "testproject"
	topicID        = "topic1"
	subscriptionID = "subscription1"

	traceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	eventType = "google.cloud.scheduler.job.v1.executed"
	pod       = "testpod"
	container = "testcontainer"
)

var brokerConfig = &config.TargetsConfig{
	Brokers: map[string]*config.Broker{
		"ns1/broker1": {
			Id:            "b-uid-1",
			Name:          "broker1",
			Namespace:     "ns1",
			DecoupleQueue: &config.Queue{Topic: topicID, State: config.State_READY},
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
		"ns4/broker4": {
			Id:            "b-uid-4",
			Name:          "broker4",
			Namespace:     "ns4",
			DecoupleQueue: &config.Queue{Topic: "topic4", State: config.State_UNKNOWN},
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
	decouple        DecoupleSink
}

type fakeOverloadedDecoupleSink struct{}

func (m *fakeOverloadedDecoupleSink) Send(ctx context.Context, broker types.NamespacedName, event cev2.Event) protocol.Result {
	return bundler.ErrOverflow
}

func TestHandler(t *testing.T) {
	tests := []testCase{
		{
			name:     "health check",
			path:     "/healthz",
			method:   nethttp.MethodGet,
			wantCode: nethttp.StatusOK,
		},
		{
			name:           "happy case",
			path:           "/ns1/broker1",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusAccepted,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
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
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "404",
				metricskey.LabelResponseCodeClass: "4xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
		{
			name:           "decouple queue not ready",
			path:           "/ns4/broker4",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusServiceUnavailable,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "503",
				metricskey.LabelResponseCodeClass: "5xx",
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
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "500",
				metricskey.LabelResponseCodeClass: "5xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
		},
		{
			name:           "pubsub overloaded",
			path:           "/ns1/broker1",
			event:          createTestEvent("test-event"),
			wantCode:       nethttp.StatusTooManyRequests,
			wantEventCount: 1,
			wantMetricTags: map[string]string{
				metricskey.LabelEventType:         eventType,
				metricskey.LabelResponseCode:      "429",
				metricskey.LabelResponseCodeClass: "4xx",
				metricskey.PodName:                pod,
				metricskey.ContainerName:          container,
			},
			decouple: &fakeOverloadedDecoupleSink{},
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

			decouple := tc.decouple
			if decouple == nil {
				decouple = NewMultiTopicDecoupleSink(ctx, memory.NewTargets(brokerConfig), createPubsubClient(ctx, t, psSrv), pubsub.DefaultPublishSettings)
			}

			url := createAndStartIngress(ctx, t, psSrv, decouple)
			rec := setupTestReceiver(ctx, t, psSrv)

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

func BenchmarkIngressHandler(b *testing.B) {
	for _, eventSize := range kgcptesting.BenchmarkEventSizes {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			runIngressHandlerBenchmark(b, eventSize)
		})
	}
}

func runIngressHandlerBenchmark(b *testing.B, eventSize int) {
	reportertest.ResetIngressMetrics()

	ctx := logging.WithLogger(context.Background(),
		zaptest.NewLogger(b,
			zaptest.WrapOptions(zap.AddCaller()),
			zaptest.Level(zap.WarnLevel),
		).Sugar())

	psSrv := pstest.NewServer()
	defer psSrv.Close()

	psClient := createPubsubClient(ctx, b, psSrv)
	decouple := NewMultiTopicDecoupleSink(ctx, memory.NewTargets(brokerConfig), psClient, pubsub.DefaultPublishSettings)
	statsReporter, err := metrics.NewIngressReporter(metrics.PodName(pod), metrics.ContainerName(container))
	if err != nil {
		b.Fatal(err)
	}
	h := NewHandler(ctx, nil, decouple, statsReporter)

	if _, err := psClient.CreateTopic(ctx, topicID); err != nil {
		b.Fatal(err)
	}

	message := binding.ToMessage(kgcptesting.NewTestEvent(b, eventSize))

	// Set parallelism to 32 so that the benchmark is not limited by the Pub/Sub publish latency
	// over gRPC.
	b.SetParallelism(32)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/ns1/broker1", nil)
			http.WriteRequest(ctx, message, req)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if res := w.Result(); w.Result().StatusCode != nethttp.StatusAccepted {
				b.Errorf("Unexpected HTTP status code %v", res.StatusCode)
			}
		}
	})
}

func createPubsubClient(ctx context.Context, t testing.TB, psSrv *pstest.Server) *pubsub.Client {
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

func setupTestReceiver(ctx context.Context, t testing.TB, psSrv *pstest.Server) *cepubsub.Protocol {
	ps := createPubsubClient(ctx, t, psSrv)
	topic, err := ps.CreateTopic(ctx, topicID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ps.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{Topic: topic})
	if err != nil {
		t.Fatal(err)
	}
	p, err := cepubsub.New(ctx,
		cepubsub.WithClient(ps),
		cepubsub.WithSubscriptionAndTopicID(subscriptionID, topicID),
		cepubsub.WithReceiveSettings(&pubsub.DefaultReceiveSettings),
	)
	if err != nil {
		t.Fatal(err)
	}

	go p.OpenInbound(cecontext.WithLogger(ctx, logtest.TestLogger(t)))
	return p
}

// createAndStartIngress creates an ingress and calls its Start() method in a goroutine.
func createAndStartIngress(ctx context.Context, t testing.TB, psSrv *pstest.Server, decouple DecoupleSink) string {
	receiver := &testHttpMessageReceiver{urlCh: make(chan string)}
	statsReporter, err := metrics.NewIngressReporter(metrics.PodName(pod), metrics.ContainerName(container))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(ctx, receiver, decouple, statsReporter)

	errCh := make(chan error, 1)
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
		metricstest.CheckStatsNotReported(t, "event_count")
	} else {
		metricstest.CheckStatsReported(t, "event_count")
		metricstest.CheckCountData(t, "event_count", tc.wantMetricTags, tc.wantEventCount)
	}
}

func assertTraceID(id string) eventAssertion {
	return func(t *testing.T, e *cloudevents.Event) {
		t.Helper()
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
