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
	"fmt"

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

func (r *Reconciler) ReconcileSubscription(ctx context.Context, id string, subConfig pubsub.SubscriptionConfig, obj runtime.Object, updater StatusUpdater) (*pubsub.Subscription, error) {
	logger := logging.FromContext(ctx)
	sub := r.client.Subscription(id)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		updater.MarkSubscriptionUnknown("SubscriptionVerificationFailed", "Failed to verify Pub/Sub subscription exists: %v", err)
		return nil, err
	}

	// Check if the topic of the subscription is "_deleted-topic_"
	if subExists {
		config, err := sub.Config(ctx)
		if err != nil {
			logger.Error("Failed to get Pub/Sub subscription Config", zap.Error(err))
			updater.MarkSubscriptionUnknown("SubscriptionConfigUnknown", "Failed to get Pub/Sub subscription Config: %v", err)
			return nil, err
		}
		if config.Topic != nil && config.Topic.String() == deletedTopic {
			logger.Error("Detected deleted topic. Going to recreate the pull subscription. Unacked messages will be lost.")
			r.recorder.Eventf(obj, corev1.EventTypeWarning, topicDeleted, "Unexpected topic deletion detected for subscription: %q", sub.ID())
			// Subscription with "_deleted-topic_" cannot pull from the new topic. In order to recover, we first delete
			// the sub and then create it. Unacked messages will be lost.
			if err := r.deleteSubscription(ctx, sub, obj); err != nil {
				updater.MarkSubscriptionFailed("SubscriptionDeletionFailed", "topic of the subscription has been deleted, need to recreate the subscription: %v", err)
				return nil, fmt.Errorf("topic of the subscription has been deleted, need to recreate the subscription: %v", err)
			}
			return r.createSubscription(ctx, id, subConfig, obj, updater)
		}
		updater.MarkSubscriptionReady()
		return sub, nil
	}

	return r.createSubscription(ctx, id, subConfig, obj, updater)
}

func (r *Reconciler) DeleteSubscription(ctx context.Context, id string, obj runtime.Object, updater StatusUpdater) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling sub")

	sub := r.client.Subscription(id)
	exists, err := sub.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		updater.MarkSubscriptionUnknown("FinalizeSubscriptionVerificationFailed", "failed to verify Pub/Sub subscription exists: %v", err)
		return err
	}
	if exists {
		if err = r.deleteSubscription(ctx, sub, obj); err != nil {
			updater.MarkSubscriptionUnknown("FinalizeSubscriptionDeletionFailed", "failed to delete Pub/Sub subscription: %v", err)
			return err
		}
	}
	return nil
}

func (r *Reconciler) deleteSubscription(ctx context.Context, sub *pubsub.Subscription, obj runtime.Object) error {
	logger := logging.FromContext(ctx)
	if err := sub.Delete(ctx); err != nil {
		logger.Error("Failed to delete Pub/Sub subscription", zap.Error(err))
		return err
	}
	logger.Info("Deleted PubSub subscription", zap.String("name", sub.ID()))
	r.recorder.Eventf(obj, corev1.EventTypeNormal, subDeleted, "Deleted PubSub subscription %q", sub.ID())
	return nil
}

func (r *Reconciler) createSubscription(ctx context.Context, id string, subConfig pubsub.SubscriptionConfig, obj runtime.Object, updater StatusUpdater) (*pubsub.Subscription, error) {
	logger := logging.FromContext(ctx)
	logger.Debug("Creating sub with cfg", zap.String("id", id), zap.Any("cfg", subConfig))
	sub, err := r.client.CreateSubscription(ctx, id, subConfig)
	if err != nil {
		logger.Error("Failed to create subscription", zap.Error(err))
		updater.MarkSubscriptionFailed("SubscriptionCreationFailed", "Subscription creation failed: %v", err)
		return nil, err
	}
	logger.Info("Created PubSub subscription", zap.String("name", sub.ID()))
	r.recorder.Eventf(obj, corev1.EventTypeNormal, subCreated, "Created PubSub subscription %q", sub.ID())
	updater.MarkSubscriptionReady()
	return sub, nil
}
