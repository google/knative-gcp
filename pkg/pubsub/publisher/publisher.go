/*
Copyright 2019 Google LLC

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

package publisher

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/zap"

	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"

	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"go.opencensus.io/trace"
)

const (
	sinkTimeout = 30 * time.Second
)

// PubSubPublisher is an interface to publish events to a pubsub topic.
type PubSubPublisher interface {
	Publish(ctx context.Context, event cev2.Event) protocol.Result
}

// HttpMessageReceiver is an interface to listen on http requests.
type HttpMessageReceiver interface {
	StartListen(ctx context.Context, handler nethttp.Handler) error
}

// Publisher receives HTTP events and sends them to Pubsub.
type Publisher struct {
	// inbound is an HTTP server to receive events.
	inbound HttpMessageReceiver
	// topic is the topic to publish events to.
	topic *pubsub.Topic

	logger *zap.Logger
}

// NewPublisher creates a new publisher.
func NewPublisher(ctx context.Context, inbound HttpMessageReceiver, topic *pubsub.Topic) *Publisher {
	return &Publisher{
		inbound: inbound,
		topic:   topic,
		logger:  logging.FromContext(ctx),
	}
}

// Start blocks to receive events over HTTP.
func (p *Publisher) Start(ctx context.Context) error {
	return p.inbound.StartListen(ctx, p)
}

// ServeHTTP implements net/http Publisher interface method.
// 1. Performs basic validation of the request.
// 2. Converts the request to an event.
// 3. Sends the event to pubsub.
func (p *Publisher) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	ctx := request.Context()
	p.logger.Debug("Serving http", zap.Any("headers", request.Header))
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	event, err := p.toEvent(request)
	if err != nil {
		nethttp.Error(response, err.Error(), nethttp.StatusBadRequest)
		return
	}

	// Optimistically set status code to StatusAccepted. It will be updated if there is an error.
	// According to the data plane spec (https://github.com/knative/eventing/blob/master/docs/spec/data-plane.md), a
	// non-callable Sink (which Publisher is) MUST respond with 202 Accepted if the request is accepted.
	statusCode := nethttp.StatusAccepted
	ctx, cancel := context.WithTimeout(ctx, sinkTimeout)
	defer cancel()
	if res := p.Publish(ctx, event); !cev2.IsACK(res) {
		msg := fmt.Sprintf("Error publishing to PubSub. event: %+v, err: %v.", event, res)
		p.logger.Error(msg)
		statusCode = nethttp.StatusInternalServerError
		nethttp.Error(response, msg, statusCode)
		return
	}
	response.WriteHeader(statusCode)
}

// Publish publishes an incoming event to a pubsub topic.
func (p *Publisher) Publish(ctx context.Context, event *cev2.Event) protocol.Result {
	dt := extensions.FromSpanContext(trace.FromContext(ctx).SpanContext())
	msg := new(pubsub.Message)
	if err := cepubsub.WritePubSubMessage(ctx, binding.ToMessage(event), msg, dt.WriteTransformer()); err != nil {
		return err
	}
	_, err := p.topic.Publish(ctx, msg).Get(ctx)
	return err
}

// toEvent converts an http request to an event.
func (p *Publisher) toEvent(request *nethttp.Request) (*cev2.Event, error) {
	message := http.NewMessageFromHttpRequest(request)
	defer func() {
		if err := message.Finish(nil); err != nil {
			p.logger.Error("Failed to close message", zap.Any("message", message), zap.Error(err))
		}
	}()
	// If encoding is unknown, the message is not an event.
	if message.ReadEncoding() == binding.EncodingUnknown {
		msg := fmt.Sprintf("Encoding is unknown. Not a cloud event? request: %+v", request)
		p.logger.Debug(msg)
		return nil, errors.New(msg)
	}
	event, err := binding.ToEvent(request.Context(), message, transformer.AddTimeNow)
	if err != nil {
		msg := fmt.Sprintf("Failed to convert request to event: %v", err)
		p.logger.Error(msg)
		return nil, errors.New(msg)
	}
	return event, nil
}
