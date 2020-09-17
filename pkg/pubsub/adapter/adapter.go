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
	nethttp "net/http"

	"go.uber.org/zap"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/extensions"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/knative-gcp/pkg/apis/messaging"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/logging"
	kntracing "knative.dev/eventing/pkg/tracing"
)

// AdapterArgs has a bundle of arguments needed to create an Adapter.
type AdapterArgs struct {
	// TopicID is the id of the Pub/Sub topic.
	TopicID string

	// SinkURI is the URI where to sink events to.
	SinkURI string

	// TransformerURI is the URI for the transformer.
	// Used for channels.
	TransformerURI string

	// Extensions is the converted ExtensionsBased64 value.
	Extensions map[string]string

	// ConverterType use to select which converter to use.
	ConverterType converters.ConverterType
}

// Adapter implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Adapter struct {
	// subscription is the pubsub subscription used to receive messages from pubsub.
	subscription *pubsub.Subscription

	// outbound is the client used to send events to.
	outbound *nethttp.Client

	// reporter reports metrics to the configured backend.
	reporter StatsReporter

	// converter used to convert pubsub messages to CE.
	converter converters.Converter

	// projectID is the id of the GCP project.
	projectID string

	// namespacedName is the namespaced name of the resource this receive adapter belong to.
	namespacedName types.NamespacedName

	// resourceGroup is the resource group name this receive adapter belong to.
	resourceGroup string

	// args holds a set of arguments used to configure the Adapter.
	args *AdapterArgs

	// cancel is function to stop pulling messages.
	cancel context.CancelFunc

	logger *zap.Logger
}

// NewAdapter creates a new adapter.
func NewAdapter(
	ctx context.Context,
	projectID clients.ProjectID,
	namespace Namespace,
	name Name,
	resourceGroup ResourceGroup,
	subscription *pubsub.Subscription,
	outbound *nethttp.Client,
	converter converters.Converter,
	reporter StatsReporter,
	args *AdapterArgs) *Adapter {
	return &Adapter{
		subscription:   subscription,
		projectID:      string(projectID),
		namespacedName: types.NamespacedName{Namespace: string(namespace), Name: string(name)},
		resourceGroup:  string(resourceGroup),
		outbound:       outbound,
		converter:      converter,
		reporter:       reporter,
		args:           args,
		logger:         logging.FromContext(ctx),
	}
}

func (a *Adapter) Start(ctx context.Context) error {
	ctx, a.cancel = context.WithCancel(ctx)

	// Augment context so that we can use it to create CE attributes.
	ctx = WithProjectKey(ctx, a.projectID)
	ctx = WithTopicKey(ctx, a.args.TopicID)
	ctx = WithSubscriptionKey(ctx, a.subscription.ID())

	return a.subscription.Receive(ctx, a.receive)
}

// Stop stops the adapter.
func (a *Adapter) Stop() {
	a.cancel()
}

// TODO refactor this method. As our RA code is used both for Sources and our Channel, it also supports replies
//  (in the case of Channels) and the logic is more convoluted.
func (a *Adapter) receive(ctx context.Context, msg *pubsub.Message) {
	event, err := a.converter.Convert(ctx, msg, a.args.ConverterType)
	if err != nil {
		a.logger.Debug("Failed to convert received message to an event, check the msg format: %v", zap.Error(err))
		// Ack the message so it won't be retried, we consider all errors to be non-retryable.
		msg.Ack()
		return
	}

	ctx, span := a.startSpan(ctx, event)
	defer span.End()

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
	if a.args.TransformerURI != "" {
		resp, err := a.sendMsg(ctx, a.args.TransformerURI, (*binding.EventMessage)(event))
		if err != nil {
			a.logger.Error("Failed to send message to transformer", zap.String("address", a.args.TransformerURI), zap.Error(err))
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
			msg.Nack()
			return
		}

		// Update the arguments used to report metrics
		args.EventType = event.Type()
		args.EventSource = event.Source()

		reply = true
	}

	// Only if the message is not from a reply, then we should add the override extensions.
	if !reply {
		// Apply CloudEvent override extensions to the outbound event.
		// This code will be mainly executed by Sources.
		for k, v := range a.args.Extensions {
			event.SetExtension(k, v)
		}
	}

	response, err := a.sendMsg(ctx, a.args.SinkURI, (*binding.EventMessage)(event))
	if err != nil {
		a.logger.Error("Failed to send message to sink", zap.String("address", a.args.SinkURI), zap.Error(err))
		msg.Nack()
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			a.logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	a.reporter.ReportEventCount(args, response.StatusCode)

	if response.StatusCode/100 != 2 {
		a.logger.Error("Event delivery failed", zap.Int("StatusCode", response.StatusCode))
		msg.Nack()
		return
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

func (a *Adapter) startSpan(ctx context.Context, event *cev2.Event) (context.Context, *trace.Span) {
	spanName := tracing.SourceDestination(a.resourceGroup, a.namespacedName)
	// This receive adapter code is used both for Sources and Channels.
	// An ugly way to identify whether it was created from a Channel is to look at the resourceGroup.
	if a.resourceGroup == messaging.ChannelsResource.String() {
		spanName = tracing.SubscriptionDestination(a.subscription.ID())
	}
	var span *trace.Span
	if dt, ok := extensions.GetDistributedTracingExtension(*event); ok {
		ctx, span = dt.StartChildSpan(ctx, spanName)
	} else {
		ctx, span = trace.StartSpan(ctx, spanName)
	}
	if span.IsRecordingEvents() {
		span.AddAttributes(
			kntracing.MessagingSystemAttribute,
			tracing.PubSubProtocolAttribute,
			kntracing.MessagingMessageIDAttribute(event.ID()),
		)
	}
	return ctx, span
}
