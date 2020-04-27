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
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/logging"
)

const (
	// TODO(liu-cong) configurable timeout
	decoupleSinkTimeout = 30 * time.Second

	// defaultPort is the defaultPort number for the ingress HTTP receiver.
	defaultPort = 8080

	// EventArrivalTime is used to access the metadata stored on a
	// CloudEvent to measure the time difference between when an event is
	// received on a broker and before it is dispatched to the trigger function.
	// The format is an RFC3339 time in string format. For example: 2019-08-26T23:38:17.834384404Z.
	EventArrivalTime = "knativearrivaltime"
)

// DecoupleSink is an interface to send events to a decoupling sink (e.g., pubsub).
type DecoupleSink interface {
	// Send sends the event from a broker to the corresponding decoupling sink.
	Send(ctx context.Context, ns, broker string, event cev2.Event) protocol.Result
}

// HttpMessageReceiver is an interface to listen on http requests.
type HttpMessageReceiver interface {
	StartListen(ctx context.Context, handler nethttp.Handler) error
}

// handler receives events and persists them to storage (pubsub).
// TODO(liu-cong) add metrics
// TODO(liu-cong) add tracing
// TODO(liu-cong) support event TTL
type handler struct {
	// httpReceiver is an HTTP server to receive events.
	httpReceiver HttpMessageReceiver
	// decouple is the client to send events to a decouple sink.
	decouple DecoupleSink
	reporter StatsReporter
	logger   *zap.Logger
}

// NewHandler creates a new ingress handler.
func NewHandler(ctx context.Context, reporter StatsReporter, options ...HandlerOption) (*handler, error) {
	h := &handler{
		reporter: reporter,
		logger:   logging.FromContext(ctx),
	}

	for _, option := range options {
		if err := option(h); err != nil {
			return nil, err
		}
	}

	if h.httpReceiver == nil {
		h.httpReceiver = kncloudevents.NewHttpMessageReceiver(defaultPort)
	}

	if h.decouple == nil {
		sink, err := NewMultiTopicDecoupleSink(ctx)
		if err != nil {
			return nil, err
		}
		h.decouple = sink
	}

	return h, nil
}

// Start blocks to receive events over HTTP.
func (h *handler) Start(ctx context.Context) error {
	return h.httpReceiver.StartListen(ctx, h)
}

// ServeHTTP implements net/http Handler interface method.
// 1. Performs basic validation of the request.
// 2. Parse request URL to get namespace and broker.
// 3. Convert request to event.
// 4. Send event to decouple sink.
func (h *handler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	h.logger.Debug("Serving http", zap.Any("headers", request.Header))
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	// Path should be in the form of "/<ns>/<broker>".
	pieces := strings.Split(request.URL.Path, "/")
	if len(pieces) != 3 {
		msg := fmt.Sprintf("Malformed request path. want: '/<ns>/<broker>'; got: %v..", request.URL.Path)
		h.logger.Info(msg)
		response.WriteHeader(nethttp.StatusNotFound)
		response.Write([]byte(msg))
		return
	}
	ns, broker := pieces[1], pieces[2]
	event, msg, statusCode := h.toEvent(request)
	defer func() { h.reportMetrics(ns, broker, event, time.Now(), statusCode) }()
	if event == nil {
		response.WriteHeader(statusCode)
		response.Write([]byte(msg))
		return
	}

	event.SetExtension(EventArrivalTime, cev2.Timestamp{Time: time.Now()})

	ctx, cancel := context.WithTimeout(request.Context(), decoupleSinkTimeout)
	defer cancel()
	if res := h.decouple.Send(ctx, ns, broker, *event); !cev2.IsACK(res) {
		msg := fmt.Sprintf("Error publishing to PubSub for broker %v/%v. event: %+v, err: %v.", ns, broker, event, res)
		h.logger.Error(msg)
		statusCode = nethttp.StatusInternalServerError
		if errors.Is(res, ErrNotFound) {
			statusCode = nethttp.StatusNotFound
		}
		response.WriteHeader(statusCode)
		response.Write([]byte(msg))
		return
	}

	response.WriteHeader(nethttp.StatusOK)
}

// toEvent converts an http request to an event.
func (h *handler) toEvent(request *nethttp.Request) (event *cev2.Event, msg string, statusCode int) {
	message := http.NewMessageFromHttpRequest(request)
	defer func() {
		if err := message.Finish(nil); err != nil {
			h.logger.Error("Failed to close message", zap.Any("message", message), zap.Error(err))
		}
	}()
	// If encoding is unknown, the message is not an event.
	if message.ReadEncoding() == binding.EncodingUnknown {
		msg := fmt.Sprintf("Encoding is unknown. Not a cloud event? request: %+v", request)
		h.logger.Debug(msg)
		return nil, msg, nethttp.StatusBadRequest
	}
	event, err := binding.ToEvent(request.Context(), message, transformer.AddTimeNow)
	if err != nil {
		msg := fmt.Sprintf("Failed to convert request to event: %v", err)
		h.logger.Error(msg)
		return nil, msg, nethttp.StatusBadRequest
	}
	return event, "", nethttp.StatusOK
}

func (h *handler) reportMetrics(ns, broker string, event *cev2.Event, start time.Time, statusCode int) {
	eventType := "Unknown"
	if event != nil {
		eventType = event.Type()
	}
	reporterArgs := &ReportArgs{
		Namespace: ns,
		Broker:    broker,
		EventType: eventType,
	}
	h.reporter.ReportEventDispatchTime(reporterArgs, statusCode, time.Since(start))
	h.reporter.ReportEventCount(reporterArgs, statusCode)
}
