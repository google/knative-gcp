/*
Copyright 2020 Google LLC

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

package pubsub

import (
	"context"
	"errors"

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/logging"
)

const (
	// If the topic of the subscription has been deleted, the value of its topic becomes "_deleted-topic_".
	// See https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#subscription
	deletedTopic = "_deleted-topic_"
	subCreated   = "SubscriptionCreated"
	subDeleted   = "SubscriptionDeleted"
)

var deletedTopicErr = errors.New("topic of the subscription has been deleted")

func (r *Reconciler) ReconcileSubscription(ctx context.Context, id string, subConfig pubsub.SubscriptionConfig, obj runtime.Object, updater StatusUpdater) (*pubsub.Subscription, error) {
	logger := logging.FromContext(ctx)
	sub := r.client.Subscription(id)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		updater.MarkSubscriptionUnknown("SubscriptionVerificationFailed", "Failed to verify Pub/Sub subscription exists: %w", err)
		return nil, err
	}

	// Check if the topic of the subscription is "_deleted-topic_"
	if subExists {
		config, err := sub.Config(ctx)
		if err != nil {
			logger.Error("Failed to get Pub/Sub subscription Config", zap.Error(err))
			updater.MarkSubscriptionUnknown("SubscriptionConfigUnknown", "Failed to get Pub/Sub subscription Config: %w", err)
			return nil, err
		}
		if config.Topic != nil && config.Topic.String() == deletedTopic {
			logger.Error("The topic is deleted")
			updater.MarkSubscriptionFailed("DeletedTopic", "Pull subscriptions must be recreated to work with recreated topic")
			return nil, deletedTopicErr
		}
		return sub, nil
	}

	// Create a new subscription.
	logger.Debug("Creating sub with cfg", zap.String("id", id), zap.Any("cfg", subConfig))
	sub, err = r.client.CreateSubscription(ctx, id, subConfig)
	if err != nil {
		logger.Error("Failed to create subscription", zap.Error(err))
		updater.MarkSubscriptionFailed("SubscriptionCreationFailed", "Subscription creation failed: %w", err)
		return nil, err
	}
	logger.Info("Created PubSub subscription", zap.String("name", sub.ID()))
	r.Recorder.Eventf(obj, corev1.EventTypeNormal, subCreated, "Created PubSub subscription %q", sub.ID())
	updater.MarkSubscriptionReady()
	return sub, nil
}

func (r *Reconciler) DeleteSubscription(ctx context.Context, id string, obj runtime.Object) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling sub")

	sub := r.client.Subscription(id)
	exists, err := sub.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		return err
	}
	if exists {
		if err := sub.Delete(ctx); err != nil {
			logger.Error("Failed to delete Pub/Sub subscription", zap.Error(err))
			return err
		}
		logger.Info("Deleted PubSub subscription", zap.String("name", sub.ID()))
		r.Recorder.Eventf(obj, corev1.EventTypeNormal, subDeleted, "Deleted PubSub subscription %q", sub.ID())
	}

	return nil
}
