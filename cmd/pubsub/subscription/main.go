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
	"cloud.google.com/go/pubsub"
	"errors"
	"flag"
	"log"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/kelseyhightower/envconfig"
	"github.com/knative/pkg/logging"
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
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// Subscription is the environment variable containing the name of the
	// subscription to use.
	Subscription string `envconfig:"PUBSUB_SUBSCRIPTION_ID" required:"true"`
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
		project, err := resolveProjectID(ctx)
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		env.Project = project
	}

	logger.Info("Pub/Sub Subscription Job.", zap.Reflect("env", env))

	client, err := pubsub.NewClient(ctx, env.Project)
	if err != nil {
		logger.Fatal("Failed to create Pub/Sub client.", zap.Error(err))
	}

	switch env.Action {
	case "create":
		_, err := getOrCreateSubscription(ctx, client, env.Topic, env.Subscription)
		if err != nil {
			logger.Fatal("Failed to create subscription.", zap.Error(err))
		}
		logger.Info("Successfully Created.", zap.String("subscription", env.Subscription))

	case "delete":
		sub := client.Subscription(env.Subscription)
		ok, err := sub.Exists(ctx)
		if err != nil {
			logger.Fatal("Failed to get subscription.", zap.Error(err))
		}
		if ok {
			if err := sub.Delete(ctx); err != nil {
				logger.Fatal("Failed to delete subscription.", zap.Error(err))
			}
		}
		logger.Info("Successfully Deleted.", zap.String("subscription", env.Subscription))

	default:
		logger.Fatal("unknown action value.", zap.String("Action", env.Action))
	}

	logger.Info("Done.")
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

func getOrCreateSubscription(ctx context.Context, client *pubsub.Client, topicID, subscriptionID string) (*pubsub.Subscription, error) {
	var err error
	// Load the topic.
	var topic *pubsub.Topic
	topic, err = getTopic(ctx, client, topicID)
	if err != nil {
		return nil, err
	}
	// Load the subscription.
	sub := client.Subscription(subscriptionID)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}
	// If subscription doesn't exist, create it.
	if !ok {
		// Create a new subscription to the previous topic with the given name.
		return client.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       30 * time.Second,
			RetentionDuration: 25 * time.Hour,
		})
	}
	return sub, nil
}

func getTopic(ctx context.Context, client *pubsub.Client, topicID string) (*pubsub.Topic, error) {
	// Load the topic.
	topic := client.Topic(topicID)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return topic, err
	}
	return nil, errors.New("topic does not exist")
}
