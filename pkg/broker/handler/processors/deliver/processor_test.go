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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	kgcptesting "github.com/google/knative-gcp/pkg/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"knative.dev/pkg/logging"
	logtest "knative.dev/pkg/logging/testing"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/broker/eventutil"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/metrics"
	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"

	_ "knative.dev/pkg/metrics/testing"
)

const (
	fakeTargetAddress  = "target"
	fakeIngressAddress = "ingress"
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
	sampleEvent := newSampleEvent()
	sampleReply := sampleEvent.Clone()
	sampleReply.SetID("reply")

	cases := []struct {
		name       string
		origin     *event.Event
		wantOrigin *event.Event
		reply      *event.Event
		wantReply  *event.Event
	}{{
		name:       "success",
		origin:     sampleEvent,
		wantOrigin: sampleEvent,
		reply:      &sampleReply,
		wantReply: func() *event.Event {
			copy := sampleReply.Clone()
			eventutil.UpdateRemainingHops(context.Background(), &copy, defaultEventHopsLimit)
			return &copy
		}(),
	}, {
		name: "success with dropped reply",
		origin: func() *event.Event {
			copy := sampleEvent.Clone()
			eventutil.UpdateRemainingHops(context.Background(), &copy, 1)
			return &copy
		}(),
		wantOrigin: sampleEvent,
		reply:      &sampleReply,
	}}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reportertest.ResetDeliveryMetrics()
			ctx := logtest.TestContextWithLogger(t)
			targetClient, err := cehttp.New()
			if err != nil {
				t.Fatalf("failed to create target cloudevents client: %v", err)
			}
			ingressClient, err := cehttp.New()
			if err != nil {
				t.Fatalf("failed to create ingress cloudevents client: %v", err)
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
			ctx = handlerctx.WithBrokerKey(ctx, broker.Key())
			ctx = handlerctx.WithTargetKey(ctx, target.Key())

			r, err := metrics.NewDeliveryReporter("pod", "container")
			if err != nil {
				t.Fatal(err)
			}
			p := &Processor{
				DeliverClient: http.DefaultClient,
				Targets:       testTargets,
				StatsReporter: r,
			}

			rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			go func() {
				msg, resp, err := targetClient.Respond(rctx)
				if err != nil && err != io.EOF {
					t.Errorf("unexpected error from target receiving event: %v", err)
				}
				if err := resp(rctx, binding.ToMessage(tc.reply), protocol.ResultACK); err != nil {
					t.Errorf("unexpected error from target responding event: %v", err)
				}
				defer msg.Finish(nil)
				gotEvent, err := binding.ToEvent(rctx, msg)
				if err != nil {
					t.Errorf("target received message cannot be converted to an event: %v", err)
				}
				if diff := cmp.Diff(tc.wantOrigin, gotEvent); diff != "" {
					t.Errorf("target received event (-want,+got): %v", diff)
				}
			}()

			go func() {
				msg, err := ingressClient.Receive(rctx)
				if err != nil && err != io.EOF {
					t.Errorf("unexpected error from ingress receiving event: %v", err)
				}
				var gotEvent *event.Event
				if msg != nil {
					defer msg.Finish(nil)
					var err error
					gotEvent, err = binding.ToEvent(rctx, msg)
					if err != nil {
						t.Errorf("ingress received message cannot be converted to an event: %v", err)
					}
					// Get and set the hops if it presents.
					// HTTP transport changes the internal type of the hops from int32 to string.
					if hops, ok := eventutil.GetRemainingHops(rctx, gotEvent); ok {
						eventutil.DeleteRemainingHops(rctx, gotEvent)
						eventutil.UpdateRemainingHops(rctx, gotEvent, hops)
					}
				}
				if diff := cmp.Diff(tc.wantReply, gotEvent); diff != "" {
					t.Errorf("ingress received event (-want,+got): %v", diff)
				}
			}()

			if err := p.Process(ctx, tc.origin); err != nil {
				t.Errorf("unexpected error from processing: %v", err)
			}

			<-rctx.Done()
		})
	}
}

type targetWithFailureHandler struct {
	t              *testing.T
	delay          time.Duration
	respCode       int
	malFormedEvent bool
}

func (h *targetWithFailureHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.t.Helper()
	_, err := ioutil.ReadAll(req.Body)
	if err != nil {
		h.t.Errorf("Failed to read request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	time.Sleep(h.delay)

	if h.malFormedEvent {
		w.Write([]byte("not an valid event body"))
	}
	w.WriteHeader(h.respCode)
}

func TestDeliverFailure(t *testing.T) {
	cases := []struct {
		name          string
		withRetry     bool
		targetHandler *targetWithFailureHandler
		failRetry     bool
		wantErr       bool
	}{{
		name:          "delivery error no retry",
		targetHandler: &targetWithFailureHandler{respCode: http.StatusInternalServerError},
		wantErr:       true,
	}, {
		name:          "delivery error retry success",
		targetHandler: &targetWithFailureHandler{respCode: http.StatusInternalServerError},
		withRetry:     true,
	}, {
		name:          "delivery error retry failure",
		targetHandler: &targetWithFailureHandler{respCode: http.StatusInternalServerError},
		withRetry:     true,
		failRetry:     true,
		wantErr:       true,
	}, {
		name:          "delivery timeout no retry",
		targetHandler: &targetWithFailureHandler{delay: time.Second, respCode: http.StatusOK},
		wantErr:       true,
	}, {
		name:          "delivery timeout retry success",
		targetHandler: &targetWithFailureHandler{delay: time.Second, respCode: http.StatusOK},
		withRetry:     true,
	}, {
		name:          "delivery timeout retry failure",
		withRetry:     true,
		targetHandler: &targetWithFailureHandler{delay: time.Second, respCode: http.StatusOK},
		failRetry:     true,
		wantErr:       true,
	}, {
		name: "malformed reply failure",
		// Return 2xx but with a malformed event should be considered error.
		targetHandler: &targetWithFailureHandler{respCode: http.StatusOK, malFormedEvent: true},
		wantErr:       true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reportertest.ResetDeliveryMetrics()
			ctx := logtest.TestContextWithLogger(t)
			tc.targetHandler.t = t
			targetSvr := httptest.NewServer(tc.targetHandler)
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

			r, err := metrics.NewDeliveryReporter("pod", "container")
			if err != nil {
				t.Fatal(err)
			}
			p := &Processor{
				DeliverClient:      http.DefaultClient,
				Targets:            testTargets,
				RetryOnFailure:     tc.withRetry,
				DeliverRetryClient: deliverRetryClient,
				DeliverTimeout:     500 * time.Millisecond,
				StatsReporter:      r,
			}

			origin := newSampleEvent()
			err = p.Process(ctx, origin)
			if (err != nil) != tc.wantErr {
				t.Errorf("processing got error=%v, want=%v", err, tc.wantErr)
			}
		})
	}
}

type NoReplyHandler struct{}

func (NoReplyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	io.Copy(ioutil.Discard, req.Body)
	w.WriteHeader(http.StatusAccepted)
}

func BenchmarkDeliveryNoReply(b *testing.B) {
	httpClient := http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: runtime.NumCPU(),
		},
	}
	targetSvr := httptest.NewServer(NoReplyHandler(struct{}{}))
	defer targetSvr.Close()

	for _, eventSize := range []int{0, 1000, 1000000} {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			benchmarkNoReply(b, &httpClient, targetSvr.URL, eventSize)
		})
	}
}

type ReplyHandler struct {
	msg binding.Message
}

func (h ReplyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	io.Copy(ioutil.Discard, req.Body)
	cehttp.WriteResponseWriter(req.Context(), h.msg, http.StatusOK, w)
}

func BenchmarkDeliveryWithReply(b *testing.B) {
	httpClient := http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: runtime.NumCPU()}}
	ingressSvr := httptest.NewServer(NoReplyHandler(struct{}{}))
	defer ingressSvr.Close()
	for _, eventSize := range []int{0, 1000, 1000000} {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			benchmarkWithReply(b, ingressSvr.URL, eventSize,
				func(b *testing.B, reply *event.Event) (http.Client, string) {
					targetSvr := httptest.NewServer(ReplyHandler{
						msg: binding.ToMessage(reply),
					})
					b.Cleanup(targetSvr.Close)
					return httpClient, targetSvr.URL
				},
			)
		})
	}
}

type fakeRoundTripper map[string]*http.Response

func (r fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		defer req.Body.Close()
	}
	if resp, ok := r[req.URL.String()]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("URL not found: %q", req.URL)
}

// BenchmarkDeliveryNoReplyFakeClient benchmarks delivery using a fake HTTP client. Use of the
// fake client helps to better isolate the performance of the processor itself and can provide
// better profiling data.
func BenchmarkDeliveryNoReplyFakeClient(b *testing.B) {
	targetAddress := "target"
	httpClient := http.Client{
		Transport: fakeRoundTripper(map[string]*http.Response{
			targetAddress: {
				StatusCode: 202,
				Header:     make(map[string][]string),
				Body:       fakeBody{new(bytes.Buffer)},
			},
		}),
	}
	for _, eventSize := range kgcptesting.BenchmarkEventSizes {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			benchmarkNoReply(b, &httpClient, targetAddress, eventSize)
		})
	}
}

type fakeBody struct {
	*bytes.Buffer
}

func (b fakeBody) Close() error {
	return nil
}

func makeFakeTargetWithReply(b *testing.B, reply *event.Event) (httpClient http.Client, targetAddress string) {
	req, err := http.NewRequest("POST", "", nil)
	if err != nil {
		b.Fatal(err)
	}
	cehttp.WriteRequest(context.Background(), binding.ToMessage(reply), req)
	body := new(bytes.Buffer)
	if req.Body != nil {
		io.Copy(body, req.Body)
		req.Body.Close()
	}

	httpClient = http.Client{
		Transport: fakeRoundTripper(map[string]*http.Response{
			fakeTargetAddress: {
				StatusCode: 200,
				Header:     req.Header,
				Body:       fakeBody{body},
			},
			fakeIngressAddress: {
				StatusCode: 202,
				Header:     make(map[string][]string),
				Body:       fakeBody{new(bytes.Buffer)},
			},
		}),
	}
	targetAddress = fakeTargetAddress
	return
}

// BenchmarkDeliveryWithReplyFakeClient benchmarks delivery with reply using a fake HTTP client. Use
// of the fake client helps to better isolate the performance of the processor itself and can
// provide better profiling data.
func BenchmarkDeliveryWithReplyFakeClient(b *testing.B) {
	for _, eventSize := range kgcptesting.BenchmarkEventSizes {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			benchmarkWithReply(b, fakeIngressAddress, eventSize, makeFakeTargetWithReply)
		})
	}
}

func benchmarkNoReply(b *testing.B, httpClient *http.Client, targetAddress string, eventSize int) {
	reportertest.ResetDeliveryMetrics()
	statsReporter, err := metrics.NewDeliveryReporter("pod", "container")
	if err != nil {
		b.Fatal(err)
	}

	sampleEvent := kgcptesting.NewTestEvent(b, eventSize)

	broker := &config.Broker{Namespace: "ns", Name: "broker"}
	target := &config.Target{Namespace: "ns", Name: "target", Broker: "broker", Address: targetAddress}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
		bm.UpsertTargets(target)
	})
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(b, zaptest.Level(zap.InfoLevel)).Sugar())
	ctx = handlerctx.WithBrokerKey(ctx, broker.Key())
	ctx = handlerctx.WithTargetKey(ctx, target.Key())

	p := &Processor{
		DeliverClient:  httpClient,
		Targets:        testTargets,
		StatsReporter:  statsReporter,
		RetryOnFailure: false,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := p.Process(ctx, sampleEvent); err != nil {
				b.Errorf("unexpected error from processing: %v", err)
			}
		}
	})
}

func benchmarkWithReply(b *testing.B, ingressAddress string, eventSize int, makeTarget func(*testing.B, *event.Event) (httpClient http.Client, targetAdress string)) {
	reportertest.ResetDeliveryMetrics()
	statsReporter, err := metrics.NewDeliveryReporter("pod", "container")
	if err != nil {
		b.Fatal(err)
	}

	sampleEvent := kgcptesting.NewTestEvent(b, eventSize)
	sampleReply := sampleEvent.Clone()
	sampleReply.SetID("reply")

	httpClient, targetAddress := makeTarget(b, &sampleReply)

	broker := &config.Broker{Namespace: "ns", Name: "broker"}
	target := &config.Target{Namespace: "ns", Name: "target", Broker: "broker", Address: targetAddress}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
		bm.SetAddress(ingressAddress)
		bm.UpsertTargets(target)
	})
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(b, zaptest.Level(zap.InfoLevel)).Sugar())
	ctx = handlerctx.WithBrokerKey(ctx, broker.Key())
	ctx = handlerctx.WithTargetKey(ctx, target.Key())

	p := &Processor{
		DeliverClient: &httpClient,
		Targets:       testTargets,
		StatsReporter: statsReporter,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := p.Process(ctx, sampleEvent); err != nil {
				b.Errorf("unexpected error from processing: %v", err)
			}
		}
	})
}

type RetryHandler struct{}

func (RetryHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	io.Copy(ioutil.Discard, req.Body)
	w.WriteHeader(http.StatusInternalServerError)
}

func BenchmarkDeliveryWithRetry(b *testing.B) {
	httpClient := http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: runtime.NumCPU(),
		},
	}
	targetSvr := httptest.NewServer(RetryHandler(struct{}{}))
	defer targetSvr.Close()

	for _, eventSize := range []int{0, 1000, 1000000} {
		b.Run(fmt.Sprintf("%d bytes", eventSize), func(b *testing.B) {
			benchmarkRetry(b, &httpClient, targetSvr.URL, eventSize)
		})
	}
}

func benchmarkRetry(b *testing.B, httpClient *http.Client, targetAddress string, eventSize int) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(b, zaptest.Level(zap.ErrorLevel)).Sugar())

	// Disable pubsub batching
	pubsub.DefaultPublishSettings.CountThreshold = 1
	_, c, close := testPubsubClient(ctx, b, "test-project")
	defer close()

	if _, err := c.CreateTopic(ctx, "test-retry-topic"); err != nil {
		b.Fatalf("failed to create test pubsub topc: %v", err)
	}

	ps, err := cepubsub.New(ctx, cepubsub.WithClient(c), cepubsub.WithProjectID("test-project"))
	if err != nil {
		b.Fatalf("failed to create pubsub protocol: %v", err)
	}
	deliverRetryClient, err := ceclient.New(ps)
	if err != nil {
		b.Fatalf("failed to create cloudevents client: %v", err)
	}

	reportertest.ResetDeliveryMetrics()
	statsReporter, err := metrics.NewDeliveryReporter("pod", "container")
	if err != nil {
		b.Fatal(err)
	}

	sampleEvent := kgcptesting.NewTestEvent(b, eventSize)

	broker := &config.Broker{Namespace: "ns", Name: "broker"}
	target := &config.Target{
		Namespace:  "ns",
		Name:       "target",
		Broker:     "broker",
		Address:    targetAddress,
		RetryQueue: &config.Queue{Topic: "test-retry-topic"},
	}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
		bm.UpsertTargets(target)
	})
	ctx = handlerctx.WithBrokerKey(ctx, broker.Key())
	ctx = handlerctx.WithTargetKey(ctx, target.Key())

	p := &Processor{
		DeliverClient:      httpClient,
		Targets:            testTargets,
		StatsReporter:      statsReporter,
		RetryOnFailure:     true,
		DeliverRetryClient: deliverRetryClient,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := p.Process(ctx, sampleEvent); err != nil {
				b.Errorf("unexpected error from processing: %v", err)
			}
		}
	})
}

func toFakePubsubMessage(m *pstest.Message) *pubsub.Message {
	return &pubsub.Message{
		ID:         m.ID,
		Attributes: m.Attributes,
		Data:       m.Data,
	}
}

func testPubsubClient(ctx context.Context, t testing.TB, projectID string) (*pstest.Server, *pubsub.Client, func()) {
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

func newSampleEvent() *event.Event {
	sampleEvent := event.New()
	sampleEvent.SetID("id")
	sampleEvent.SetSource("source")
	sampleEvent.SetSubject("subject")
	sampleEvent.SetType("type")
	sampleEvent.SetTime(time.Now())
	return &sampleEvent
}
