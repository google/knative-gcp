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

package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/google/knative-gcp/pkg/testing/testloggingutil"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/tracing"

	. "github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	tracingconfig "github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/kelseyhightower/envconfig"
)

const (
	component = "receive_adapter"

	// TODO make this configurable
	maxConnectionsPerHost = 1000
)

// TODO we should refactor this and reduce the number of environment variables.
//  most of them are due to metrics, which has to change anyways.
type envConfig struct {
	// Environment variable containing the sink URI.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the transformer URI.
	// This environment variable is only set if we configure both subscriber and reply
	// in Channel's subscribers.
	// If subscriber and reply are used, we map:
	//   Transformer to sub.subscriber
	//	 Sink to sub.reply
	// Otherwise, only Sink is used (for either the sub.reply or sub.reply)
	Transformer string `envconfig:"TRANSFORMER_URI"`

	// Environment variable specifying the type of adapter to use.
	// Used for CE conversion.
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
	ResourceGroup string `envconfig:"RESOURCE_GROUP" default:"pullsubscriptions.internal.pubsub.cloud.google.com" required:"true"`
}

// TODO try to use the common main from broker.
func main() {
	appcredentials.MustExistOrUnsetEnv()

	flag.Parse()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		panic(fmt.Sprintf("Failed to process env var: %s", err))
	}

	// Convert json logging.Config to logging.Config.
	loggingConfig, err := logging.JSONToConfig(env.LoggingConfigJson)
	if err != nil {
		fmt.Printf("Failed to process logging config: %s", err.Error())
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}

	sl, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := sl.Desugar()
	defer flush(logger)
	ctx := logging.WithLogger(signals.NewContext(), logger.Sugar())

	// This is added purely for the TestCloudLogging E2E tests, which verify that the log line is
	// written based on environment variables.
	testloggingutil.LogBasedOnEnv(logger)

	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JSONToOptions(env.MetricsConfigJson)
	if err != nil {
		logger.Error("Failed to process metrics options", zap.Error(err))
	}

	if metricsConfig != nil {
		if err := metrics.UpdateExporter(ctx, *metricsConfig, logger.Sugar()); err != nil {
			logger.Fatal("Failed to create the metrics exporter", zap.Error(err))
		}
	}

	tracingConfig, err := tracingconfig.JSONToConfig(env.TracingConfigJson)
	if err != nil {
		logger.Error("Failed to process tracing options", zap.Error(err))
	}
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracingConfig); err != nil {
		logger.Error("Failed to setup tracing", zap.Error(err), zap.Any("tracingConfig", tracingConfig))
	}

	projectID, err := utils.ProjectIDOrDefault("")
	if err != nil {
		logger.Fatal("Failed to retrieve project id", zap.Error(err))
	}

	// Convert base64 encoded json map to extensions map.
	extensions, err := utils.Base64ToMap(env.ExtensionsBase64)
	if err != nil {
		logger.Error("Failed to convert base64 extensions to map: %v", zap.Error(err))
	}

	logger.Info("Initializing adapter", zap.String("projectID", projectID), zap.String("topicID", env.Topic), zap.String("subscriptionID", env.Subscription))

	args := &AdapterArgs{
		TopicID:        env.Topic,
		ConverterType:  converters.ConverterType(env.AdapterType),
		SinkURI:        env.Sink,
		TransformerURI: env.Transformer,
		Extensions:     extensions,
	}

	adapter, err := InitializeAdapter(ctx,
		clients.MaxConnsPerHost(maxConnectionsPerHost),
		clients.ProjectID(projectID),
		SubscriptionID(env.Subscription),
		Namespace(env.Namespace),
		Name(env.Name),
		ResourceGroup(env.ResourceGroup),
		args)

	if err != nil {
		logger.Fatal("Unable to create adapter", zap.Error(err))
	}

	logger.Info("Starting Receive Adapter.", zap.String("projectID", projectID), zap.String("topicID", env.Topic), zap.String("subscriptionID", env.Subscription))
	if err := adapter.Start(ctx); err != nil {
		logger.Error("Adapter has stopped with error", zap.String("projectID", projectID), zap.String("topicID", env.Topic), zap.String("subscriptionID", env.Subscription), zap.Error(err))
	} else {
		logger.Error("Adapter has stopped", zap.String("projectID", projectID), zap.String("topicID", env.Topic), zap.String("subscriptionID", env.Subscription), zap.Error(err))
	}

	// Wait a grace period for the handlers to shutdown.
	time.Sleep(30 * time.Second)
	logger.Info("Exiting...")
}

func flush(logger *zap.Logger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
