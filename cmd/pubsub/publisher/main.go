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
	"context"
	"flag"
	"log"

	. "github.com/google/knative-gcp/pkg/pubsub/publisher"
	"github.com/google/knative-gcp/pkg/testing/testloggingutil"
	tracingconfig "github.com/google/knative-gcp/pkg/tracing"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/appcredentials"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"knative.dev/pkg/tracing"
)

type envConfig struct {
	// Environment variable containing the port for the publisher.
	Port int `envconfig:"PORT" default:"8080"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// TracingConfigJson is a JSON string of tracing.Config. This is used to configure tracing. The
	// original config is stored in a ConfigMap inside the controller's namespace. Its value is
	// copied here as a JSON string.
	TracingConfigJson string `envconfig:"K_TRACING_CONFIG" required:"true"`
}

func main() {
	appcredentials.MustExistOrUnsetEnv()

	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig() // TODO: to replace with a dynamically updating logger.
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	// This is added purely for the TestCloudLogging E2E tests, which verify that the log line is
	// written based on environment variables.
	testloggingutil.LogBasedOnEnv(logger)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	projectID, err := utils.ProjectIDOrDefault("")
	if err != nil {
		logger.Fatal("Failed to retrieve project id", zap.Error(err))
	}

	topicID := env.Topic
	if topicID == "" {
		logger.Fatal("Failed to retrieve topic id", zap.Error(err))
	}

	tracingConfig, err := tracingconfig.JSONToConfig(env.TracingConfigJson)
	if err != nil {
		logger.Error("Failed to process tracing options", zap.Error(err))
	}
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracingConfig); err != nil {
		logger.Error("Failed to setup tracing", zap.Error(err), zap.Any("tracingConfig", tracingConfig))
	}

	logger.Info("Initializing publisher", zap.String("Project ID", projectID), zap.String("Topic ID", topicID))

	publisher, err := InitializePublisher(
		ctx,
		clients.Port(env.Port),
		clients.ProjectID(projectID),
		TopicID(topicID),
	)

	if err != nil {
		logger.Fatal("Unable to create publisher", zap.Error(err))
	}

	logger.Info("Starting publisher", zap.Any("publisher", publisher))
	if err := publisher.Start(ctx); err != nil {
		logger.Error("Publisher has stopped with error", zap.String("Project ID", projectID), zap.String("Topic ID", topicID), zap.Error(err))
	}
}
