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
	"log"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
)

type envConfig struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// Action is the operation the job should run.
	// Options: [create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// created. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`
}

// TODO: the job could output the resolved projectID.

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

	logger = logger.With(
		zap.String("action", env.Action),
		zap.String("project", env.Project),
		zap.String("topic", env.Topic),
	)

	logger.Info("Pub/Sub Topic Job.")

	client, err := pubsub.NewClient(ctx, env.Project)
	if err != nil {
		logger.Fatal("Failed to create Pub/Sub client.", zap.Error(err))
	}

	topic := client.Topic(env.Topic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		logger.Fatal("Failed to verify topic exists.", zap.Error(err))
	}

	switch env.Action {
	case "create":
		// If topic doesn't exist, create it.
		if !exists {
			// Create a new topic with the given name.
			topic, err = client.CreateTopic(ctx, env.Topic)
			if err != nil {
				logger.Fatal("Failed to create topic.", zap.Error(err))
			}
			logger.Info("Successfully created.")
		} else {
			// TODO: here is where we could update topic config.
			logger.Info("Previously created.")
		}

	case "delete":
		if exists {
			if err := topic.Delete(ctx); err != nil {
				logger.Fatal("Failed to delete topic.", zap.Error(err))
			}
			logger.Info("Successfully deleted.")
		} else {
			logger.Info("Previously deleted.")
		}

	default:
		logger.Fatal("Unknown action value.")
	}

	logger.Info("Done.")
}
