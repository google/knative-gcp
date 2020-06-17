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

package adapter

import (
	"context"
	"errors"
	nethttp "net/http"
	"time"

	"go.uber.org/zap"

	"cloud.google.com/go/pubsub"
	"github.com/cloudevents/sdk-go/v2/binding"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"go.opencensus.io/trace"
	"knative.dev/eventing/pkg/logging"
)

// Adapter implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Adapter struct {
	// subscription is the pubsub subscription used to receive messages from pubsub.
	subscription *pubsub.Subscription

	// outbound is the client used to send events to.
	outbound *nethttp.Client

	// sinkURI is the URI where to sink events to.
	sinkURI string

	// transformerURI is the URI for the transformer.
	// Used for channels.
	transformerURI string

	// extensions is the converted ExtensionsBased64 value.
	extensions map[string]string

	// adapterType use to select which converter to use.
	adapterType string

	// reporter reports metrics to the configured backend.
	reporter StatsReporter

	// converter used to convert pubsub messages to CE.
	converter converters.Converter

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc

	logger *zap.Logger
}

// NewAdapter creates a new adapter.
func NewAdapter(
	ctx context.Context,
	subscription *pubsub.Subscription,
	outbound *nethttp.Client,
	converter converters.Converter,
	reporter StatsReporter,
	sinkURI SinkURI,
	transformerURI TransformerURI,
	adapterType AdapterType,
	extensions map[string]string) *Adapter {
	return &Adapter{
		subscription:   subscription,
		outbound:       outbound,
		converter:      converter,
		reporter:       reporter,
		sinkURI:        string(sinkURI),
		transformerURI: string(transformerURI),
		adapterType:    string(adapterType),
		extensions:     extensions,
		logger:         logging.FromContext(ctx),
	}
}

func (a *Adapter) Start(ctx context.Context) error {
	ctx, a.cancel = context.WithCancel(ctx)
	defer a.cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.subscription.Receive(ctx, a.receive)
	}()

	// Stop either if the adapter stops (sending to errCh) or if the context Done channel is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		break
	}

	// Done channel has been closed, we need to gracefully shutdown. The cancel() method will start its
	// shutdown, if it hasn't finished in a reasonable amount of time, just return an error.
	a.cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(30 * time.Second):
		return errors.New("timeout shutting down adapter")
	}
}

// Stop stops the adapter.
func (a *Adapter) Stop() {
	a.cancel()
}

// TODO refactor this method. As our RA code is used both for Sources and our Channel, it also supports replies
//  (in the case of Channels) and the logic is more convoluted.
func (a *Adapter) receive(ctx context.Context, msg *pubsub.Message) {
	event, err := a.converter.Convert(ctx, msg, a.adapterType)
	if err != nil {
		a.logger.Debug("Failed to convert received message to an event, check the msg format: %w", zap.Error(err))
		// Ack the message so it won't be retried
		msg.Ack()
		return
	}

	args := &ReportArgs{
		EventType:   event.Type(),
		EventSource: event.Source(),
	}

	// Using this variable to check whether the event came from a reply or not.
	reply := false

	// If a transformer has been configured, then "transform" the message.
	// Note that currently this path of the code will be executed when using the receive adapter as part of the underlying Channel,
	// in case both subscriber and reply are set. The transformer would act as the subscriber and the sink will be where
	// we will send the reply.
	if a.transformerURI != "" {
		resp, err := a.sendMsg(ctx, a.transformerURI, (*binding.EventMessage)(event))
		if err != nil {
			a.logger.Error("Failed to send message to transformer", zap.String("address", a.transformerURI), zap.Error(err))
			msg.Nack()
			return
		}

		defer func() {
			if err := resp.Body.Close(); err != nil {
				a.logger.Warn("Failed to close response body", zap.Error(err))
			}
		}()

		a.reporter.ReportEventCount(args, resp.StatusCode)

		if resp.StatusCode/100 != 2 {
			a.logger.Error("Event delivery failed", zap.Int("StatusCode", resp.StatusCode))
			msg.Nack()
			return
		}

		respMsg := cehttp.NewMessageFromHttpResponse(resp)
		if respMsg.ReadEncoding() == binding.EncodingUnknown {
			// No reply
			msg.Ack()
			return
		}

		// If there was a reply, we need to send it to the sink.
		// We then overwrite the initial event we sent.
		event, err = binding.ToEvent(ctx, respMsg)
		if err != nil {
			a.logger.Error("Failed to convert response message to event",
				zap.Any("response", respMsg), zap.Error(err))
			msg.Ack()
			return
		}

		// TODO check if this is OK
		// Update the tracing information to use the span returned by the transformer.
		// ctx = trace.NewContext(ctx, trace.FromContext(transformedCTX))
		if span := trace.FromContext(ctx); span.IsRecordingEvents() {
			span.Annotate(ceclient.EventTraceAttributes(event),
				"Event reply",
			)
		}

		reply = true
	}

	// Only if the message is not from a reply, then we should add the override extensions.
	if !reply {
		// Apply CloudEvent override extensions to the outbound event.
		// This code will be mainly executed by Sources.
		for k, v := range a.extensions {
			event.SetExtension(k, v)
		}
	}

	resp, err := a.sendMsg(ctx, a.sinkURI, (*binding.EventMessage)(event))
	if err != nil {
		a.logger.Error("Failed to send message to sink", zap.String("address", a.sinkURI), zap.Error(err))
		msg.Nack()
		return
	}

	a.reporter.ReportEventCount(args, resp.StatusCode)

	if resp.StatusCode/100 != 2 {
		a.logger.Error("Event delivery failed", zap.Int("StatusCode", resp.StatusCode))
		msg.Nack()
		return
	}

	if err := resp.Body.Close(); err != nil {
		a.logger.Warn("Failed to close response body", zap.Error(err))
	}

	msg.Ack()
}

func (a *Adapter) sendMsg(ctx context.Context, address string, msg binding.Message) (*nethttp.Response, error) {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, address, nil)
	if err != nil {
		return nil, err
	}
	if err := cehttp.WriteRequest(ctx, msg, req); err != nil {
		return nil, err
	}
	return a.outbound.Do(req)
}
