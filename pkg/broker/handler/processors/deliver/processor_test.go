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
	ceBody             = `{
  "specversion": "1.0",
  "type": "com.example.someevent",
  "id": "123",
  "source": "http://example.com/",
  "data": {
    "foo": "bar"
  }
}
`
)

func TestInvalidContext(t *testing.T) {
	p := &Processor{Targets: memory.NewEmptyTargets()}
	e := event.New()
	err := p.Process(context.Background(), &e)
	if err != handlerctx.ErrBrokerKeyNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrBrokerKeyNotPresent)
	}

	ctx := handlerctx.WithBrokerKey(context.Background(), config.TestOnlyBrokerKey("ns", "does-not-exist"))
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

			broker := &config.CellTenant{
				Type:      config.CellTenantType_BROKER,
				Namespace: "ns",
				Name:      "broker",
			}
			target := &config.Target{
				Namespace:      "ns",
				Name:           "target",
				CellTenantType: config.CellTenantType_BROKER,
				CellTenantName: "broker",
				Address:        targetSvr.URL,
			}
			testTargets := memory.NewEmptyTargets()
			testTargets.MutateCellTenant(broker.Key(), func(bm config.CellTenantMutation) {
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
	t                     *testing.T
	delay                 time.Duration
	structuredContentMode bool
	nonCloudEventReply    bool
	respCode              int
	respBody              string
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

	w.Header().Set("ce-specversion", "1.0")
	if h.nonCloudEventReply {
		// missing ce-specversion header
		w.Header().Del("ce-specversion")
		// Content-Type does not start with `application/cloudevents`
		w.Header().Set("Content-Type", "text/html")
	} else if h.structuredContentMode {
		w.Header().Set("Content-Type", "application/cloudevents+json; charset=utf-8")
	}
	w.WriteHeader(h.respCode)
	w.Write([]byte(h.respBody))
}

// statusCodeReplyHandler is intended to be used as the handler that replies are sent to. It will
// respond with `responseCode`. If `responseCode` is not set, then the handler asserts that it
// should not have been called (i.e. no reply event was expected to be sent).
type statusCodeReplyHandler struct {
	t            *testing.T
	responseCode int
	eventsSeen   int
}

func (h *statusCodeReplyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	io.Copy(ioutil.Discard, req.Body)
	h.eventsSeen += 1
	if h.responseCode == 0 {
		h.t.Errorf("Reply Handler was not configured, no event should have been seen")
	} else {
		w.WriteHeader(h.responseCode)
	}
}

func TestProcess_WithoutSubscriberAddress(t *testing.T) {
	testCases := []struct {
		name                string
		cellTenantType      config.CellTenantType
		replyHandler        statusCodeReplyHandler
		expectedReplyEvents int
		error               string
	}{
		{
			name:           "trigger",
			cellTenantType: config.CellTenantType_BROKER,
			error:          "trigger ns/target has no subscriber address",
		},
		{
			name:           "Channel",
			cellTenantType: config.CellTenantType_CHANNEL,
			replyHandler: statusCodeReplyHandler{
				responseCode: http.StatusAccepted,
			},
			expectedReplyEvents: 1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reportertest.ResetDeliveryMetrics()
			ctx := logtest.TestContextWithLogger(t)
			_, c, close := testPubsubClient(ctx, t, "test-project")
			defer close()
			replySvr := httptest.NewServer(&tc.replyHandler)
			defer replySvr.Close()

			ps, err := cepubsub.New(ctx, cepubsub.WithClient(c), cepubsub.WithProjectID("test-project"))
			if err != nil {
				t.Fatalf("failed to create pubsub protocol: %v", err)
			}
			deliverRetryClient, err := ceclient.New(ps)
			if err != nil {
				t.Fatalf("failed to create cloudevents client: %v", err)
			}

			ct := &config.CellTenant{
				Type:      tc.cellTenantType,
				Namespace: "ns",
				Name:      "ct",
				Address:   replySvr.URL,
			}
			target := &config.Target{
				Namespace:      ct.Namespace,
				Name:           "target",
				CellTenantType: ct.Type,
				CellTenantName: ct.Name,
				Address:        "",
				ReplyAddress:   replySvr.URL,
				RetryQueue: &config.Queue{
					Topic: "test-retry-topic",
				},
			}
			testTargets := memory.NewEmptyTargets()
			testTargets.MutateCellTenant(ct.Key(), func(bm config.CellTenantMutation) {
				bm.SetAddress(ct.Address)
				bm.UpsertTargets(target)
			})
			ctx = handlerctx.WithBrokerKey(ctx, ct.Key())
			ctx = handlerctx.WithTargetKey(ctx, target.Key())

			r, err := metrics.NewDeliveryReporter("pod", "container")
			if err != nil {
				t.Fatal(err)
			}
			p := &Processor{
				DeliverClient:      http.DefaultClient,
				Targets:            testTargets,
				DeliverRetryClient: deliverRetryClient,
				DeliverTimeout:     500 * time.Millisecond,
				StatsReporter:      r,
			}

			origin := newSampleEvent()
			err = p.Process(ctx, origin)
			if wantErr := (tc.error != ""); wantErr {
				if err == nil {
					t.Error("Wanted an error, received nil")
				} else if tc.error != err.Error() {
					t.Errorf("Unexpected error, want %q, got %q", tc.error, err.Error())
				}
			} else if err != nil {
				t.Errorf("Wanted no error, received %v", err)
			}
			if want, got := tc.expectedReplyEvents, tc.replyHandler.eventsSeen; want != got {
				t.Errorf("Unexpected number of reply events. Want %d, Got %d", want, got)
			}
		})
	}
}

func TestDeliverFailure(t *testing.T) {
	cases := []struct {
		name                string
		withRetry           bool
		targetHandler       *targetWithFailureHandler
		replyHandler        statusCodeReplyHandler
		expectedReplyEvents int
		failRetry           bool
		wantErr             bool
	}{{
		name:          "delivery error no retry",
		targetHandler: &targetWithFailureHandler{respCode: http.StatusInternalServerError},
		wantErr:       true,
	}, {
		name:          "delivery error retry success",
		targetHandler: &targetWithFailureHandler{respCode: http.StatusInternalServerError},
		withRetry:     true,
		wantErr:       false,
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
		wantErr:       false,
	}, {
		name:          "delivery timeout retry failure",
		withRetry:     true,
		targetHandler: &targetWithFailureHandler{delay: time.Second, respCode: http.StatusOK},
		failRetry:     true,
		wantErr:       true,
	}, {
		name: "malformed CloudEvent reply failure",
		// Return 2xx but with a malformed event should be considered error.
		targetHandler: &targetWithFailureHandler{
			respCode:              http.StatusOK,
			respBody:              "not a valid structured cloud event",
			structuredContentMode: true,
		},
		wantErr: true,
	}, {
		name: "non-CloudEvent reply success",
		// a non-CloudEvent reply with 2xx status code should be considered delivery success.
		targetHandler: &targetWithFailureHandler{
			respCode:           http.StatusOK,
			respBody:           "reply body",
			nonCloudEventReply: true,
		},
		wantErr: false,
	}, {
		name: "non-CloudEvent reply failure",
		// a non-CloudEvent reply with non-2xx status code should be considered delivery failure.
		targetHandler: &targetWithFailureHandler{
			respCode:           http.StatusBadRequest,
			respBody:           "reply body",
			nonCloudEventReply: true,
		},
		wantErr: true,
	}, {
		name: "reply server failure",
		targetHandler: &targetWithFailureHandler{
			respCode: http.StatusAccepted,
			respBody: ceBody,
		},
		replyHandler: statusCodeReplyHandler{
			responseCode: http.StatusBadRequest,
		},
		expectedReplyEvents: 1,
		wantErr:             true,
	}, {
		name: "reply server success",
		targetHandler: &targetWithFailureHandler{
			respCode: http.StatusAccepted,
			respBody: ceBody,
		},
		replyHandler: statusCodeReplyHandler{
			responseCode: http.StatusOK,
		},
		expectedReplyEvents: 1,
		wantErr:             false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reportertest.ResetDeliveryMetrics()
			ctx := logtest.TestContextWithLogger(t)
			tc.targetHandler.t = t
			targetSvr := httptest.NewServer(tc.targetHandler)
			defer targetSvr.Close()
			tc.replyHandler.t = t
			replySvr := httptest.NewServer(&tc.replyHandler)
			defer replySvr.Close()

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

			broker := &config.CellTenant{
				Type:      config.CellTenantType_BROKER,
				Namespace: "ns",
				Name:      "broker",
				Address:   replySvr.URL,
			}
			target := &config.Target{
				Namespace:      "ns",
				Name:           "target",
				CellTenantType: config.CellTenantType_BROKER,
				CellTenantName: "broker",
				Address:        targetSvr.URL,
				ReplyAddress:   replySvr.URL,
				RetryQueue: &config.Queue{
					Topic: "test-retry-topic",
				},
			}
			testTargets := memory.NewEmptyTargets()
			testTargets.MutateCellTenant(broker.Key(), func(bm config.CellTenantMutation) {
				bm.SetAddress(broker.Address)
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
			if want, got := tc.expectedReplyEvents, tc.replyHandler.eventsSeen; want != got {
				t.Errorf("Unexpected number of reply events. Want %d, Got %d", want, got)
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

	broker := &config.CellTenant{Namespace: "ns", Name: "broker"}
	target := &config.Target{
		Namespace:      "ns",
		Name:           "target",
		CellTenantType: config.CellTenantType_BROKER,
		CellTenantName: "broker",
		Address:        targetAddress,
	}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateCellTenant(broker.Key(), func(bm config.CellTenantMutation) {
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

	broker := &config.CellTenant{Namespace: "ns", Name: "broker"}
	target := &config.Target{
		Namespace:      "ns",
		Name:           "target",
		CellTenantType: config.CellTenantType_BROKER,
		CellTenantName: "broker",
		Address:        targetAddress,
	}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateCellTenant(broker.Key(), func(bm config.CellTenantMutation) {
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

	broker := &config.CellTenant{Namespace: "ns", Name: "broker"}
	target := &config.Target{
		Namespace:      "ns",
		Name:           "target",
		CellTenantType: config.CellTenantType_BROKER,
		CellTenantName: "broker",
		Address:        targetAddress,
		RetryQueue:     &config.Queue{Topic: "test-retry-topic"},
	}
	testTargets := memory.NewEmptyTargets()
	testTargets.MutateCellTenant(broker.Key(), func(bm config.CellTenantMutation) {
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
	sampleEvent.SetSpecVersion("1.0")
	sampleEvent.SetType("type")
	sampleEvent.SetTime(time.Now())
	return &sampleEvent
}
