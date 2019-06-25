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

package operations

import (
	"context"
	"errors"
	"time"

	"github.com/knative/pkg/kmeta"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SubArgs are the configuration required to make a NewSubscriptionOps.
type SubArgs struct {
	Image          string
	Action         string
	ProjectID      string
	TopicID        string
	SubscriptionID string
	Owner          kmeta.OwnerRefable
}

// NewSubscriptionOps returns a new batch Job resource.
func NewSubscriptionOps(arg SubArgs) *batchv1.Job {
	podTemplate := makePodTemplate(arg.Image, []corev1.EnvVar{{
		Name:  "ACTION",
		Value: arg.Action,
	}, {
		Name:  "PROJECT_ID",
		Value: arg.ProjectID,
	}, {
		Name:  "PUBSUB_TOPIC_ID",
		Value: arg.TopicID,
	}, {
		Name:  "PUBSUB_SUBSCRIPTION_ID",
		Value: arg.SubscriptionID,
	}}...)

	backoffLimit := int32(3)
	parallelism := int32(1)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    SubscriptionJobName(arg.Owner, arg.Action),
			Namespace:       arg.Owner.GetObjectMeta().GetNamespace(),
			Labels:          SubscriptionJobLabels(arg.Owner, arg.Action),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(arg.Owner)},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Template:     *podTemplate,
		},
	}
}

// TODO: the job could output the resolved projectID.

// SubscriptionOps defines the configuration to use for this operation.
type SubscriptionOps struct {
	PubSubOps

	// Action is the operation the job should run.
	// Options: [exists, create, delete]
	Action string `envconfig:"ACTION" required:"true"`

	// Topic is the environment variable containing the PubSub Topic being
	// subscribed to's name. In the form that is unique within the project.
	// E.g. 'laconia', not 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"PUBSUB_TOPIC_ID" required:"true"`

	// Subscription is the environment variable containing the name of the
	// subscription to use.
	Subscription string `envconfig:"PUBSUB_SUBSCRIPTION_ID" required:"true"`
}

// Run will perform the action configured upon a subscription.
func (s *SubscriptionOps) Run(ctx context.Context) error {
	if s.Client == nil {
		return errors.New("pub/sub client is nil")
	}
	logger := logging.FromContext(ctx)

	logger = logger.With(
		zap.String("action", s.Action),
		zap.String("project", s.Project),
		zap.String("topic", s.Topic),
		zap.String("subscription", s.Subscription),
	)

	logger.Info("Pub/Sub Subscription Job.")

	// Load the subscription.
	sub := s.Client.Subscription(s.Subscription)
	exists, err := sub.Exists(ctx)
	if err != nil {
		logger.Fatal("Failed to verify topic exists.", zap.Error(err))
	}

	switch s.Action {
	case ActionExists:
		// If subscription doesn't exist, that is an error.
		if !exists {
			logger.Fatal("Subscription does not exist.")
		}
		logger.Info("Previously created.")

	case ActionCreate:
		// If topic doesn't exist, create it.
		if !exists {
			// Load the topic.
			topic, err := s.getTopic(ctx)
			if err != nil {
				logger.Fatal("Failed to get topic.", zap.Error(err))
			}
			// Create a new subscription to the previous topic with the given name.
			sub, err = s.Client.CreateSubscription(ctx, s.Subscription, pubsub.SubscriptionConfig{
				Topic:             topic,
				AckDeadline:       30 * time.Second,
				RetentionDuration: 25 * time.Hour,
			})
			if err != nil {
				logger.Fatal("Failed to create subscription.", zap.Error(err))
			}
			logger.Info("Successfully created.")
		} else {
			// TODO: here is where we could update config.
			logger.Info("Previously created.")
		}

	case ActionDelete:
		if exists {
			if err := sub.Delete(ctx); err != nil {
				logger.Fatal("Failed to delete subscription.", zap.Error(err))
			}
			logger.Info("Successfully deleted.")
		} else {
			logger.Info("Previously deleted.")
		}

	default:
		logger.Fatal("unknown action value.")
	}

	logger.Info("Done.")
	return nil
}

func (s *SubscriptionOps) getTopic(ctx context.Context) (pubsub.Topic, error) {
	// Load the topic.
	topic := s.Client.Topic(s.Topic)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return topic, err
	}
	return nil, errors.New("topic does not exist")
}
