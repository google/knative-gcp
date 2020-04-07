package ingress

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	cev2 "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/volume"
	"github.com/google/knative-gcp/pkg/utils"
	"knative.dev/eventing/pkg/logging"
)

// NewMultiTopicDecoupleSink creates a new multiTopicDecoupleSink.
func NewMultiTopicDecoupleSink(ctx context.Context, options ...MultiTopicDecoupleSinkOption) (*multiTopicDecoupleSink, error) {
	sink := &multiTopicDecoupleSink{
		logger: logging.FromContext(ctx),
	}

	for _, opt := range options {
		opt(sink)
	}

	// Apply defaults
	if sink.client == nil {
		client, err := newDefaultPubSubClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create pubsub client: %v", err)
		}
		sink.client = client
	}
	if sink.brokerConfig == nil {
		brokerConfig, err := volume.NewTargetsFromFile()
		if err != nil {
			return nil, fmt.Errorf("creating broker config for default multi topic decouple sink")
		}
		sink.brokerConfig = brokerConfig
	}

	return sink, nil
}

// multiTopicDecoupleSink implements DecoupleSink and routes events to pubsub topics corresponding
// to the broker to which the events are sent.
type multiTopicDecoupleSink struct {
	// client talks to pubsub.
	client cev2.Client
	// brokerConfig holds configurations for all brokers. It's a view of a configmap populated by
	// the broker controller.
	brokerConfig config.ReadonlyTargets
	logger       *zap.Logger
}

// Send sends incoming event to its corresponding pubsub topic based on which broker it belongs to.
func (m *multiTopicDecoupleSink) Send(ctx context.Context, ns, broker string, event cev2.Event) protocol.Result {
	topic, err := m.getTopicForBroker(ns, broker)
	if err != nil {
		return err
	}
	ctx = cecontext.WithTopic(ctx, topic)
	return m.client.Send(ctx, event)
}

// getTopicForBroker finds the corresponding decouple topic for the broker from the mounted broker configmap volume.
func (m *multiTopicDecoupleSink) getTopicForBroker(ns, broker string) (string, error) {
	brokerConfig, ok := m.brokerConfig.GetBroker(ns, broker)
	if !ok {
		// There is an propagation delay between the controller reconciles the broker config and
		// the config being pushed to the configmap volume in the ingress pod. So sometimes we return
		// an error even if the request is valid.
		m.logger.Warn("config is not found for", zap.Any("ns", ns), zap.Any("broker", broker))
		return "", fmt.Errorf("%q/%q: %w", ns, broker, ErrNotFound)
	}
	if brokerConfig.DecoupleQueue == nil || brokerConfig.DecoupleQueue.Topic == "" {
		m.logger.Error("DecoupleQueue or topic missing for broker, this should NOT happen.", zap.Any("brokerConfig", brokerConfig))
		return "", fmt.Errorf("decouple queue of %q/%q: %w", ns, broker, ErrIncomplete)
	}
	return brokerConfig.DecoupleQueue.Topic, nil
}

// newDefaultPubSubClient creates a pubsub client using project ID from GCE metadata. Project ID can be overridden by
// env var "GOOGLE_CLOUD_PROJECT".
func newDefaultPubSubClient(ctx context.Context) (cev2.Client, error) {
	project, err := utils.ProjectID(os.Getenv(pubsub.DefaultProjectEnvKey))
	if err != nil {
		return nil, err
	}
	// Make a pubsub protocol for the CloudEvents client.
	p, err := pubsub.New(ctx, pubsub.WithProjectID(project))
	if err != nil {
		return nil, err
	}

	// Use the pubsub prototol to make a new CloudEvents client.
	return cev2.NewClientObserved(p,
		cev2.WithUUIDs(),
		cev2.WithTimeNow(),
		cev2.WithTracePropagation,
	)
}
