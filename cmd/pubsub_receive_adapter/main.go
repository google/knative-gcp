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
	"errors"
	"flag"
	"log"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"

	gcppubsub "github.com/GoogleCloudPlatform/cloud-run-events/pkg/adapter"
)

type envConfig struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID" required:"true"`

	// Environment variable containing the sink URI.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the transformer URI.
	Transformer string `envconfig:"TRANSFORMER_URI"`

	// Mode is the mode the receive adapter should run in.
	// Options: [start, delete]
	Mode string `envconfig:"MODE" default:"start"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// Subscription is the environment variable containing the name of the
	// subscription to use.
	Subscription string `envconfig:"PUBSUB_SUBSCRIPTION_ID" required:"true"`
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
		project, err := resolveProjectID(ctx)
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		env.Project = project
	}

	logger.Info("using project.", zap.String("project", env.Project))

	adapter := &gcppubsub.Adapter{
		ProjectID:      env.Project,
		TopicID:        env.Topic,
		SinkURI:        env.Sink,
		SubscriptionID: env.Subscription,
		TransformerURI: env.Transformer,
	}

	switch env.Mode {
	case "start":
		logger.Info("Starting Pub/Sub Receive Adapter.", zap.Reflect("adapter", adapter))
		if err := adapter.Start(ctx); err != nil {
			logger.Fatal("failed to start adapter: ", zap.Error(err))
		}

	case "delete":
		logger.Info("Deleting Pub/Sub Receive Adapter.", zap.Reflect("adapter", adapter))
		if err := adapter.DeletePubSubSubscription(ctx); err != nil {
			logger.Fatal("failed to delete adapter: ", zap.Error(err))
		}
		logger.Info("Successful.")
		// Block until done.
		<-ctx.Done()

	default:
		logger.Fatal("unknown MODE value.", zap.String("MODE", env.Mode))
	}
}

func resolveProjectID(ctx context.Context) (string, error) {
	logger := logging.FromContext(ctx)

	// Try from metadata server.
	if projectID, err := metadata.ProjectID(); err == nil {
		return projectID, nil
	} else {
		logger.Warnw("failed to get Project ID from GCP Metadata Server.", zap.Error(err))
	}

	// Unknown Project ID
	return "", errors.New("project is required but not set")
}
