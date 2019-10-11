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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"

	"knative.dev/eventing/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/google/knative-gcp/pkg/pubsub/publisher"
)

type envConfig struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

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
	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig() // TODO: to replace with a dynamically updating logger.
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	if env.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		env.Project = project
	}

	logger.Info("Using project.", zap.String("project", env.Project))

	tracingConfig, err := JsonToTracingConfig(env.TracingConfigJson)
	if err != nil {
		logger.Error("Failed to process tracing options", zap.Error(err))
	}
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracingConfig); err != nil {
		logger.Error("Failed to setup tracing", zap.Error(err), zap.Any("tracingConfig", tracingConfig))
	}

	startable := &publisher.Publisher{
		ProjectID: env.Project,
		TopicID:   env.Topic,
	}

	logger.Info("Starting Pub/Sub Publisher.", zap.Any("publisher", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start publisher: ", zap.Error(err))
	}
}

func JsonToTracingConfig(jsonConfig string) (*tracingconfig.Config, error) {
	var cfg tracingconfig.Config
	if jsonConfig == "" {
		return nil, errors.New("tracing config json string is empty")
	}

	if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling tracing config json: %v", err)
	}

	return &cfg, nil
}
