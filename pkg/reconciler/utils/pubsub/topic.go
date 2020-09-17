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

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/logging"
)

const (
	topicCreated = "TopicCreated"
	topicDeleted = "TopicDeleted"
)

func (r *Reconciler) ReconcileTopic(ctx context.Context, id string, topicConfig *pubsub.TopicConfig, obj runtime.Object, updater StatusUpdater) (*pubsub.Topic, error) {
	logger := logging.FromContext(ctx)

	// Check if topic exists, and if not, create it.
	topic := r.client.Topic(id)
	exists, err := topic.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		updater.MarkTopicUnknown("TopicVerificationFailed", "Failed to verify Pub/Sub topic exists: %v", err)
		return nil, err
	}
	if exists {
		updater.MarkTopicReady()
		return topic, nil
	}

	// Create a new topic.
	logger.Debug("Creating topic with cfg", zap.String("id", id), zap.Any("cfg", topicConfig))
	topic, err = r.client.CreateTopicWithConfig(ctx, id, topicConfig)
	if err != nil {
		logger.Error("Failed to create Pub/Sub topic", zap.Error(err))
		updater.MarkTopicFailed("TopicCreationFailed", "Topic creation failed: %v", err)
		return nil, err
	}
	logger.Info("Created PubSub topic", zap.String("name", topic.ID()))
	r.recorder.Eventf(obj, corev1.EventTypeNormal, topicCreated, "Created PubSub topic %q", topic.ID())
	updater.MarkTopicReady()
	return topic, nil
}

func (r *Reconciler) DeleteTopic(ctx context.Context, id string, obj runtime.Object, updater StatusUpdater) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling topic")

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topic := r.client.Topic(id)
	exists, err := topic.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		updater.MarkTopicUnknown("FinalizeTopicVerificationFailed", "failed to verify Pub/Sub topic exists: %v", err)
		return err
	}
	if exists {
		if err := topic.Delete(ctx); err != nil {
			logger.Error("Failed to delete Pub/Sub topic", zap.Error(err))
			updater.MarkTopicUnknown("FinalizeTopicDeletionFailed", "failed to delete Pub/Sub topic: %v", err)
			return err
		}
		logger.Info("Deleted PubSub topic", zap.String("name", topic.ID()))
		r.recorder.Eventf(obj, corev1.EventTypeNormal, topicDeleted, "Deleted PubSub topic %q", topic.ID())
	}
	return nil
}
