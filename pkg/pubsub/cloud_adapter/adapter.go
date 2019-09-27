package cloud_adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"net/url"
	"strings"

	pubsubadapter "github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	decoratorresources "github.com/google/knative-gcp/pkg/reconciler/decorator/resources"
)

// Target is passed as a url query param in the from: adapter?target=SINK_URL_AS_BASE64

// Adapter implements the Pub/Sub adapter to deliver Pub/Sub messages from a
// pre-existing topic/subscription to a Sink.
type Adapter struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Port contains the required port to start on.
	Port int `envconfig:"PORT" required:"true" default:"8080"`

	// ExtensionsBase64 is a based64 encoded json string of a map of
	// CloudEvents extensions (key-value pairs) override onto the outbound
	// event.
	ExtensionsBase64 string `envconfig:"K_CE_EXTENSIONS"`

	// extensions is the converted ExtensionsBased64 value.
	extensions map[string]string

	// SendMode describes how the adapter sends events.
	// One of [binary, structured]. Default: binary
	SendMode converters.ModeType `envconfig:"SEND_MODE" required:"true" default:"binary"`

	// inbound is the cloudevents client to use to receive events.
	inbound cloudevents.Client

	// codec is how we are going to hack in deocding from http as if it is pubsub.
	codec *pubsub.Codec

	// outbound is the cloudevents client to use to send events.
	outbound cloudevents.Client
}

// Start starts the adapter. Note: Only call once, not thread safe.
func (a *Adapter) Start(ctx context.Context) error {
	var err error

	if a.SendMode == "" {
		a.SendMode = converters.DefaultSendMode
	}

	// Convert base64 encoded json map to extensions map.
	// This implementation comes from the Decorator object.
	a.extensions, err = decoratorresources.Base64ToMap(a.ExtensionsBase64)
	if err != nil {
		fmt.Printf("[warn] failed to convert base64 extensions to map: %v", err)
	}

	// Receive Events on Pub/Sub.
	if a.inbound == nil {
		if a.inbound, err = a.newHTTPClient(ctx, true, cloudevents.WithPort(a.Port)); err != nil {
			return fmt.Errorf("failed to create inbound cloudevent client: %s", err.Error())
		}
	}

	// Send events on HTTP.
	if a.outbound == nil {
		if a.outbound, err = a.newHTTPClient(ctx, false); err != nil {
			return fmt.Errorf("failed to create outbound cloudevent client: %s", err.Error())
		}
	}

	if a.codec == nil {
		a.codec = &pubsub.Codec{}
	}

	return a.inbound.StartReceiver(ctx, a.receive)
}

func (a *Adapter) receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("event.id", event.ID()))

	// Apply CloudEvent override extensions to the outbound event.
	for k, v := range a.extensions {
		event.SetExtension(k, v)
	}

	logger.Info("sending event")

	httpCtx := cloudevents.HTTPTransportContextFrom(ctx)

	if invokedURI, err := url.Parse(httpCtx.URI); err == nil {
		targetBase64 := invokedURI.Query().Get("target")
		if targetBase64 != "" {
			if target, err := base64.StdEncoding.DecodeString(targetBase64); err == nil {
				ctx = cloudevents.ContextWithTarget(ctx, strings.TrimSpace(string(target)))
			}
		}
	}

	// Send the event and report the count.
	_, r, err := a.outbound.Send(ctx, event)
	if err != nil {
		return err
	} else if r != nil {
		resp.RespondWith(200, r)
	}
	return nil
}

func (a *Adapter) convert(ctx context.Context, m transport.Message, err error) (*cloudevents.Event, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("Converting event from transport.")

	if msg, ok := m.(*http.Message); ok {
		pm := pubsubadapter.PushMessage{}

		httpCtx := cloudevents.HTTPTransportContextFrom(ctx)

		logger.Info("CONVERT::", httpCtx, "\n\n", msg.Header, "\n\n", string(msg.Body))

		if err := json.Unmarshal(msg.Body, &pm); err != nil {
			return nil, err
		}

		psmsg := pubsub.Message{
			Attributes: pm.Message.Attributes,
			Data:       pm.Message.Data,
		}
		event, err := a.codec.Decode(ctx, psmsg)
		if err != nil {
			tctx := pubsubcontext.TransportContext{
				ID:           pm.Message.ID,
				PublishTime:  pm.Message.PublishTime,
				Project:      a.Project,
				Topic:        "TODO-Lookup",
				Subscription: pm.Subscription,
				Method:       "push",
			}
			psctx := pubsubcontext.WithTransportContext(ctx, tctx)
			// try again.
			return converters.Convert(psctx, &psmsg, a.SendMode)
		}
		return event, nil
	}
	return nil, err
}

func (a *Adapter) newHTTPClient(ctx context.Context, convert bool, tOpts ...http.Option) (cloudevents.Client, error) {
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

	if convert {
		return cloudevents.NewClient(t, cloudevents.WithConverterFn(a.convert))
	} else {
		return cloudevents.NewClient(t)
	}
}
