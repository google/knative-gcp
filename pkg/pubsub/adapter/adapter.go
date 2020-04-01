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
	"fmt"

	nethttp "net/http"

	"go.opencensus.io/trace"
	"go.uber.org/zap"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"github.com/cloudevents/sdk-go/v1/cloudevents/transport"
	"github.com/cloudevents/sdk-go/v1/cloudevents/transport/http"
	cepubsub "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub"
	"github.com/google/knative-gcp/pkg/kncloudevents"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	"github.com/google/knative-gcp/pkg/utils"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/logging"
)

var (
	channelGVR = schema.GroupVersionResource{
		Group:    "messaging.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "channels",
	}
)

// Adapter implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Adapter struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Environment variable containing the sink URI.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the transformer URI.
	Transformer string `envconfig:"TRANSFORMER_URI"`

	// Environment variable specifying the type of adapter to use.
	AdapterType string `envconfig:"ADAPTER_TYPE"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// Subscription is the environment variable containing the name of the
	// subscription to use.
	Subscription string `envconfig:"PUBSUB_SUBSCRIPTION_ID" required:"true"`

	// ExtensionsBase64 is a based64 encoded json string of a map of
	// CloudEvents extensions (key-value pairs) override onto the outbound
	// event.
	ExtensionsBase64 string `envconfig:"K_CE_EXTENSIONS" required:"true"`

	// extensions is the converted ExtensionsBased64 value.
	extensions map[string]string

	// SendMode describes how the adapter sends events.
	// One of [binary, structured, push]. Default: binary
	SendMode converters.ModeType `envconfig:"SEND_MODE" default:"binary" required:"true"`

	// MetricsConfigJson is a json string of metrics.ExporterOptions.
	// This is used to configure the metrics exporter options, the config is
	// stored in a config map inside the controllers namespace and copied here.
	MetricsConfigJson string `envconfig:"K_METRICS_CONFIG" required:"true"`

	// LoggingConfigJson is a json string of logging.Config.
	// This is used to configure the logging config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	LoggingConfigJson string `envconfig:"K_LOGGING_CONFIG" required:"true"`

	// TracingConfigJson is a JSON string of tracing.Config. This is used to configure tracing. The
	// original config is stored in a ConfigMap inside the controller's namespace. Its value is
	// copied here as a JSON string.
	TracingConfigJson string `envconfig:"K_TRACING_CONFIG" required:"true"`

	// Environment variable containing the namespace.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// Environment variable containing the name.
	Name string `envconfig:"NAME" required:"true"`

	// Environment variable containing the resource group. E.g., storages.events.cloud.google.com.
	ResourceGroup string `envconfig:"RESOURCE_GROUP" default:"pullsubscriptions.pubsub.cloud.google.com" required:"true"`

	// inbound is the cloudevents client to use to receive events.
	inbound cloudevents.Client

	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client

	// transformer is the cloudevents client to transform received events before sending.
	transformer cloudevents.Client

	// reporter reports metrics to the configured backend.
	reporter StatsReporter
}

// Start starts the adapter. Note: Only call once, not thread safe.
func (a *Adapter) Start(ctx context.Context) error {
	var err error

	if a.SendMode == "" {
		a.SendMode = converters.DefaultSendMode
	}

	// Convert base64 encoded json map to extensions map.
	a.extensions, err = utils.Base64ToMap(a.ExtensionsBase64)
	if err != nil {
		fmt.Printf("[warn] failed to convert base64 extensions to map: %v", err)
	}

	// Receive Events on Pub/Sub.
	if a.inbound == nil {
		if a.inbound, err = a.newPubSubClient(ctx); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %w", err)
		}
	}

	// Send events on HTTP.
	if a.outbound == nil {
		if a.outbound, err = a.newHTTPClient(ctx, a.Sink); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %w", err)
		}
	}

	if a.reporter == nil {
		a.reporter = NewStatsReporter()
	}

	// Make the transformer client in case the TransformerURI has been set.
	if a.Transformer != "" {
		if a.transformer == nil {
			if a.transformer, err = kncloudevents.NewDefaultClient(a.Transformer); err != nil {
				return fmt.Errorf("failed to create transformer cloudevent client: %w", err)
			}
		}
	}

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Adapter) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()), zap.Any("sink", a.Sink))

	// TODO Name and ResourceGroup might cause problems in the near future, as we might use a single receive-adapter
	//  for multiple source objects. Same with Namespace, when doing multi-tenancy.
	args := &ReportArgs{
		Name:          a.Name,
		Namespace:     a.Namespace,
		EventType:     event.Type(),
		EventSource:   event.Source(),
		ResourceGroup: a.ResourceGroup,
	}

	var err error
	// If a transformer has been configured, then transform the message.
	// Note that this path in the code will be executed when using the receive adapter as part of the underlying Channel
	// of a Broker. We currently set the TransformerURI to be the address of the Broker filter pod.
	// TODO consider renaming transformer as it is confusing.
	if a.transformer != nil {
		transformedCTX, transformedEvent, err := a.transformer.Send(ctx, event)
		rtctx := cloudevents.HTTPTransportContextFrom(transformedCTX)
		if err != nil {
			logger.Errorf("error transforming cloud event %q", event.ID())
			a.reporter.ReportEventCount(args, rtctx.StatusCode)
			return err
		}
		if transformedEvent == nil {
			// This doesn't mean there was an error. E.g., the Broker filter pod might not return a response.
			// Report the returned Status Code and return.
			logger.Debugf("cloud event %q was not transformed", event.ID())
			a.reporter.ReportEventCount(args, rtctx.StatusCode)
			return nil
		}
		// Update the event with the transformed one.
		event = *transformedEvent
		// Update the tracing information to use the span returned by the transformer.
		ctx = trace.NewContext(ctx, trace.FromContext(transformedCTX))
	}

	// Apply CloudEvent override extensions to the outbound event.
	for k, v := range a.extensions {
		event.SetExtension(k, v)
	}

	// Send the event and report the count.
	rctx, r, err := a.outbound.Send(ctx, event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	a.reporter.ReportEventCount(args, rtctx.StatusCode)
	if err != nil {
		return err
	} else if r != nil {
		resp.RespondWith(nethttp.StatusOK, r)
	}
	return nil
}

func (a *Adapter) convert(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("Converting event from transport.")

	if msg, ok := m.(*cepubsub.Message); ok {
		return converters.Convert(ctx, msg, a.SendMode, a.AdapterType)
	}
	return nil, err
}

func (a *Adapter) newPubSubClient(ctx context.Context) (cloudevents.Client, error) {
	tOpts := []cepubsub.Option{
		cepubsub.WithProjectID(a.Project),
		cepubsub.WithTopicID(a.Topic),
		cepubsub.WithSubscriptionAndTopicID(a.Subscription, a.Topic),
	}

	// Make a pubsub transport for the CloudEvents client.
	t, err := cepubsub.New(ctx, tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	return cloudevents.NewClient(t,
		cloudevents.WithConverterFn(a.convert),
	)
}

func (a *Adapter) newHTTPClient(ctx context.Context, target string) (cloudevents.Client, error) {
	tOpts := []http.Option{
		cloudevents.WithTarget(target),
	}

	switch a.SendMode {
	case converters.Binary, converters.Push:
		tOpts = append(tOpts, cloudevents.WithBinaryEncoding())
	case converters.Structured:
		tOpts = append(tOpts, cloudevents.WithStructuredEncoding())
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTPTransport(tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	return cloudevents.NewClient(t)
}
