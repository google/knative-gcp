package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	nethttp "net/http"
	"strconv"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
)

var topic = "topic1"

var brokerConfig = &config.TargetsConfig{
	Brokers: map[string]*config.Broker{
		"ns1/broker1": {
			Id:        "b-uid-1",
			Name:      "broker1",
			Namespace: "ns1",
			DecoupleQueue: &config.Queue{
				Topic: topic,
			},
		},
	},
}

type testCase struct {
	name    string
	options []HandlerOption
	// If port is different than the default(8080), a matching WithPort option should be provided.
	port int
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
			port:     defaultPort,
			wantCode: nethttp.StatusOK,
		},
		{
			name:     "with custom port number",
			path:     "/ns1/broker1",
			event:    createTestEvent("test-event"),
			port:     8081,
			options:  []HandlerOption{WithPort(8081)},
			wantCode: nethttp.StatusOK,
		},
		{
			name:     "valid event but unsupported http  method",
			method:   "PUT",
			path:     "/ns1/broker1",
			event:    createTestEvent("test-event"),
			port:     defaultPort,
			wantCode: nethttp.StatusMethodNotAllowed,
		},
		{
			name:     "malformed path",
			path:     "/ns1/broker1/and/something/else",
			event:    createTestEvent("test-event"),
			port:     defaultPort,
			wantCode: nethttp.StatusNotFound,
		},
		{
			name:     "request is not an event",
			path:     "/ns1/broker1",
			port:     defaultPort,
			wantCode: nethttp.StatusBadRequest,
			header:   nethttp.Header{},
		},
		{
			name:     "wrong path - broker doesn't exist in config",
			path:     "/ns-1/broker2",
			event:    createTestEvent("test-event"),
			port:     defaultPort,
			wantCode: nethttp.StatusInternalServerError,
		},
	}

	client := nethttp.Client{}
	defer client.CloseIdleConnections()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := createAndStartIngress(t, tc)
			defer cleanup()

			res, err := client.Do(createRequest(tc))
			if err != nil {
				t.Fatalf("Unexpected error from http client: %v", err)
			}
			if res.StatusCode != tc.wantCode {
				t.Errorf("StatusCode mismatch. got: %v, want: %v", res.StatusCode, tc.wantCode)
			}

			// If event is accepted, check that it's stored in the decouple sink.
			if res.StatusCode == nethttp.StatusOK {
				// Retrieve the event from the decouple sink.
				fakeClient := h.decouple.(*multiTopicDecoupleSink).client.(*fakePubsubClient)
				savedToSink := <-fakeClient.topics[topic]
				if tc.event.ID() != savedToSink.ID() {
					t.Errorf("Event ID mismatch. got: %v, want: %v", savedToSink.ID(), tc.event.ID())
				}
				if savedToSink.Time().IsZero() {
					t.Errorf("Saved event should be decorated with timestamp, got zero.")
				}
			}
		})
	}
}

// createAndStartIngress creates an ingress and calls its Start() method in a goroutine.
func createAndStartIngress(t *testing.T, tc testCase) (h *handler, cleanup func()) {
	decouple, err1 := NewMultiTopicDecoupleSink(context.Background(),
		WithBrokerConfig(memory.NewTargets(brokerConfig)),
		WithPubsubClient(newFakePubsubClient(t)))
	if err1 != nil {
		t.Fatalf("Failed to create decouple sink: %v", err1)
	}
	opts := append(tc.options, WithDecoupleSink(decouple))
	ctx := context.Background()
	h, err2 := NewHandler(ctx, opts...)
	if err2 != nil {
		t.Fatalf("Failed to create ingress handler: %+v", err2)
	}
	go h.Start(ctx)
	cleanup = func() {
		// Any cleanup steps should go here. For now none.
	}
	return h, cleanup
}

func createTestEvent(id string) *cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(id)
	event.SetSource("test-source")
	event.SetType("test-type")
	return &event
}

// createRequest creates an http request from the test case. If event is specified, it converts the event to a request.
func createRequest(tc testCase) *nethttp.Request {
	method := "POST"
	if tc.method != "" {
		method = tc.method
	}
	url := "http://localhost:" + strconv.Itoa(tc.port) + tc.path
	body, _ := json.Marshal(tc.body)
	request, _ := nethttp.NewRequest(method, url, bytes.NewBuffer(body))
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
