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

// TODO(liu-cong) configurable timeout
const decoupleSinkTimeout = 30 * time.Second

// defaultPort is the defaultPort number for the ingress HTTP receiver.
const defaultPort = 8080

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
	logger   *zap.Logger
}

// NewHandler creates a new ingress handler.
func NewHandler(ctx context.Context, options ...HandlerOption) (*handler, error) {
	h := &handler{
		logger: logging.FromContext(ctx),
	}

	for _, option := range options {
		option(h)
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
	h.logger.Debug("Serving http", zap.Any("request", request))
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	// Path should be in the form of "/<ns>/<broker>".
	pieces := strings.Split(request.URL.Path, "/")
	if len(pieces) != 3 {
		h.logger.Info("Malformed request path", zap.String("path", request.URL.Path))
		response.WriteHeader(nethttp.StatusNotFound)
		return
	}
	ns, broker := pieces[1], pieces[2]

	event, statusCode := h.toEvent(request)
	if event == nil {
		response.WriteHeader(statusCode)
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), decoupleSinkTimeout)
	defer cancel()
	if res := h.decouple.Send(ctx, ns, broker, *event); !cev2.IsACK(res) {
		h.logger.Error("Error publishing to PubSub", zap.String("event", event.String()), zap.String("ns", ns), zap.String("broker", broker), zap.Error(res))
		statusCode := nethttp.StatusInternalServerError
		if errors.Is(res, ErrNotFound) {
			statusCode = nethttp.StatusNotFound
		}
		response.WriteHeader(statusCode)
		return
	}

	response.WriteHeader(nethttp.StatusOK)
}

// toEvent converts an http request to an event.
func (h *handler) toEvent(request *nethttp.Request) (event *cev2.Event, statusCode int) {
	message := http.NewMessageFromHttpRequest(request)
	defer func() {
		if err := message.Finish(nil); err != nil {
			h.logger.Error("Failed to close message", zap.Any("message", message), zap.Error(err))
		}
	}()
	// If encoding is unknown, the message is not an event.
	if message.ReadEncoding() == binding.EncodingUnknown {
		h.logger.Debug("Encoding is unknown. Not a cloud event?", zap.Any("request", request))
		return nil, nethttp.StatusBadRequest
	}
	event, err := binding.ToEvent(request.Context(), message, transformer.AddTimeNow)
	if err != nil {
		h.logger.Error("Failed to convert request to event", zap.Error(err))
		return nil, nethttp.StatusBadRequest
	}
	return event, nethttp.StatusOK
}
