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
	"time"

	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/metrics"
	"github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/wire"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	grpccode "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/logging"
	kntracing "knative.dev/eventing/pkg/tracing"
)

const (
	// TODO(liu-cong) configurable timeout
	decoupleSinkTimeout = 30 * time.Second

	// EventArrivalTime is used to access the metadata stored on a
	// CloudEvent to measure the time difference between when an event is
	// received on a broker and before it is dispatched to the trigger function.
	// The format is an RFC3339 time in string format. For example: 2019-08-26T23:38:17.834384404Z.
	EventArrivalTime = "knativearrivaltime"

	// For probes.
	heathCheckPath = "/healthz"

	// for permission denied error msg
	// TODO(cathyzhyi) point to official doc rather than github doc
	deniedErrMsg string = `Failed to publish to PubSub because permission denied.
Please refer to "Configure the Authentication Mechanism for GCP" at https://github.com/google/knative-gcp/blob/master/docs/install/install-gcp-broker.md`
)

// HandlerSet provides a handler with a real HTTPMessageReceiver and pubsub MultiTopicDecoupleSink.
var HandlerSet wire.ProviderSet = wire.NewSet(
	NewHandler,
	clients.NewHTTPMessageReceiver,
	wire.Bind(new(HttpMessageReceiver), new(*kncloudevents.HttpMessageReceiver)),
	NewMultiTopicDecoupleSink,
	wire.Bind(new(DecoupleSink), new(*multiTopicDecoupleSink)),
	clients.NewPubsubClient,
	metrics.NewIngressReporter,
)

// DecoupleSink is an interface to send events to a decoupling sink (e.g., pubsub).
type DecoupleSink interface {
	// Send sends the event from a broker to the corresponding decoupling sink.
	Send(ctx context.Context, broker types.NamespacedName, event cev2.Event) protocol.Result
}

// HttpMessageReceiver is an interface to listen on http requests.
type HttpMessageReceiver interface {
	StartListen(ctx context.Context, handler nethttp.Handler) error
}

// Handler receives events and persists them to storage (pubsub).
type Handler struct {
	// httpReceiver is an HTTP server to receive events.
	httpReceiver HttpMessageReceiver
	// decouple is the client to send events to a decouple sink.
	decouple DecoupleSink
	logger   *zap.Logger
	reporter *metrics.IngressReporter
}

// NewHandler creates a new ingress handler.
func NewHandler(ctx context.Context, httpReceiver HttpMessageReceiver, decouple DecoupleSink, reporter *metrics.IngressReporter) *Handler {
	return &Handler{
		httpReceiver: httpReceiver,
		decouple:     decouple,
		reporter:     reporter,
		logger:       logging.FromContext(ctx),
	}
}

// Start blocks to receive events over HTTP.
func (h *Handler) Start(ctx context.Context) error {
	return h.httpReceiver.StartListen(ctx, h)
}

// ServeHTTP implements net/http Handler interface method.
// 1. Performs basic validation of the request.
// 2. Parse request URL to get namespace and broker.
// 3. Convert request to event.
// 4. Send event to decouple sink.
func (h *Handler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	if request.URL.Path == heathCheckPath {
		response.WriteHeader(nethttp.StatusOK)
		return
	}

	ctx := request.Context()
	ctx = logging.WithLogger(ctx, h.logger)
	ctx = tracing.WithLogging(ctx, trace.FromContext(ctx))
	logging.FromContext(ctx).Debug("Serving http", zap.Any("headers", request.Header))
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	broker, err := ConvertPathToNamespacedName(request.URL.Path)
	if err != nil {
		logging.FromContext(ctx).Debug("Malformed request path", zap.String("path", request.URL.Path))
		nethttp.Error(response, err.Error(), nethttp.StatusNotFound)
		return
	}

	event, err := h.toEvent(ctx, request)
	if err != nil {
		nethttp.Error(response, err.Error(), nethttp.StatusBadRequest)
		return
	}

	event.SetExtension(EventArrivalTime, cev2.Timestamp{Time: time.Now()})

	span := trace.FromContext(ctx)
	span.SetName(kntracing.BrokerMessagingDestination(broker))
	if span.IsRecordingEvents() {
		span.AddAttributes(
			append(
				ceclient.EventTraceAttributes(event),
				kntracing.MessagingSystemAttribute,
				tracing.PubSubProtocolAttribute,
				kntracing.BrokerMessagingDestinationAttribute(broker),
				kntracing.MessagingMessageIDAttribute(event.ID()),
			)...,
		)
	}

	// Optimistically set status code to StatusAccepted. It will be updated if there is an error.
	// According to the data plane spec (https://github.com/knative/eventing/blob/master/docs/spec/data-plane.md), a
	// non-callable SINK (which broker is) MUST respond with 202 Accepted if the request is accepted.
	statusCode := nethttp.StatusAccepted
	ctx, cancel := context.WithTimeout(ctx, decoupleSinkTimeout)
	defer cancel()
	defer func() { h.reportMetrics(request.Context(), broker, event, statusCode) }()
	if res := h.decouple.Send(ctx, broker, *event); !cev2.IsACK(res) {
		logging.FromContext(ctx).Error("Error publishing to PubSub", zap.String("broker", broker.String()), zap.Error(res))
		statusCode = nethttp.StatusInternalServerError

		switch {
		case errors.Is(res, ErrNotFound):
			statusCode = nethttp.StatusNotFound
		case errors.Is(res, ErrNotReady):
			statusCode = nethttp.StatusServiceUnavailable
		case grpcstatus.Code(res) == grpccode.PermissionDenied:
			nethttp.Error(response, deniedErrMsg, statusCode)
			return
		}
		nethttp.Error(response, "Failed to publish to PubSub", statusCode)
		return
	}

	response.WriteHeader(statusCode)
}

// toEvent converts an http request to an event.
func (h *Handler) toEvent(ctx context.Context, request *nethttp.Request) (*cev2.Event, error) {
	message := http.NewMessageFromHttpRequest(request)
	defer func() {
		if err := message.Finish(nil); err != nil {
			logging.FromContext(ctx).Error("Failed to close message", zap.Any("message", message), zap.Error(err))
		}
	}()
	// If encoding is unknown, the message is not an event.
	if message.ReadEncoding() == binding.EncodingUnknown {
		logging.FromContext(ctx).Debug("Unknown encoding", zap.Any("request", request))
		return nil, errors.New("Unknown encoding. Not a cloud event?")
	}
	event, err := binding.ToEvent(request.Context(), message, transformer.AddTimeNow)
	if err != nil {
		logging.FromContext(ctx).Error("Failed to convert request to event", zap.Error(err))
		return nil, err
	}
	return event, nil
}

func (h *Handler) reportMetrics(ctx context.Context, broker types.NamespacedName, event *cev2.Event, statusCode int) {
	args := metrics.IngressReportArgs{
		Namespace:    broker.Namespace,
		Broker:       broker.Name,
		EventType:    event.Type(),
		ResponseCode: statusCode,
	}
	if err := h.reporter.ReportEventCount(ctx, args); err != nil {
		logging.FromContext(ctx).Warn("Failed to record metrics.", zap.Any("namespace", broker.Namespace), zap.Any("broker", broker.Name), zap.Error(err))
	}
}
