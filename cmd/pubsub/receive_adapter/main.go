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

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/adapter"
)

type envConfig struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Environment variable containing the sink URI.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the transformer URI.
	Transformer string `envconfig:"TRANSFORMER_URI"`

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

	sl, _ := logging.NewLogger("", "INFO") // TODO: use logging config map.
	logger := sl.Desugar()
	defer logger.Sync()

	ctx := logging.WithLogger(signals.NewContext(), logger.Sugar())

	startable := adapter.Adapter{}
	if err := envconfig.Process("", &startable); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	if startable.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		startable.Project = project
	}

	logger.Info("using project.", zap.String("project", startable.Project))

	logger.Info("Starting Pub/Sub Receive Adapter.", zap.Any("adapter", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}
