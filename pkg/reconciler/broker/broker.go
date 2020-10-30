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

// Package broker implements the Broker controller and reconciler reconciling
// Brokers and Triggers.
package broker

import (
	"context"
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	pkgreconciler "knative.dev/pkg/reconciler"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	inteventslisters "github.com/google/knative-gcp/pkg/client/listers/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerReconciled  = "BrokerReconciled"
	brokerFinalized   = "BrokerFinalized"
	brokerCellCreated = "BrokerCellCreated"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	brokerCellLister inteventslisters.BrokerCellLister

	projectID string

	// pubsubClient is used as the Pubsub client when present.
	pubsubClient *pubsub.Client

	dataresidencyStore *dataresidency.Store
}

// Check that Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)
var _ brokerreconciler.Finalizer = (*Reconciler)(nil)

var brokerGVK = brokerv1beta1.SchemeGroupVersion.WithKind("Broker")

func (r *Reconciler) ReconcileKind(ctx context.Context, b *brokerv1beta1.Broker) pkgreconciler.Event {
	if err := r.reconcileBroker(ctx, b); err != nil {
		logging.FromContext(ctx).Error("Problem reconciling broker", zap.Error(err))
		return fmt.Errorf("failed to reconcile broker: %w", err)
		//TODO instead of returning on error, update the data plane configmap with
		// whatever info is available. or put this in a defer?
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerReconciled, "Broker reconciled: \"%s/%s\"", b.Namespace, b.Name)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *brokerv1beta1.Broker) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Debug("Finalizing Broker", zap.Any("broker", b))

	if err := r.deleteDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("failed to delete Pub/Sub topic: %v", err)
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerFinalized, "Broker finalized: \"%s/%s\"", b.Namespace, b.Name)
}

func (r *Reconciler) reconcileBroker(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling Broker", zap.Any("broker", b))
	b.Status.InitializeConditions()
	b.Status.ObservedGeneration = b.Generation

	if err := r.ensureBrokerCellExists(ctx, b); err != nil {
		return fmt.Errorf("brokercell reconcile failed: %v", err)
	}

	// Create decoupling topic and pullsub for this broker. Ingress will push
	// to this topic and fanout will pull from the pull sub.
	if err := r.reconcileDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("decoupling topic reconcile failed: %v", err)
	}

	return nil
}

func (r *Reconciler) reconcileDecouplingTopicAndSubscription(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling decoupling topic", zap.Any("broker", b))
	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectIDOrDefault(r.projectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		b.Status.MarkTopicUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		b.Status.MarkSubscriptionUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}
	// Set the projectID in the status.
	//TODO uncomment when eventing webhook allows this
	//b.Status.ProjectID = projectID

	client, err := r.getClientOrCreateNew(ctx, projectID, b)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	labels := map[string]string{
		"resource":     "brokers",
		"broker_class": brokerv1beta1.BrokerClass,
		"namespace":    b.Namespace,
		"name":         b.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}

	// Check if topic exists, and if not, create it.
	topicID := resources.GenerateDecouplingTopicName(b)
	topicConfig := &pubsub.TopicConfig{Labels: labels}
	if r.dataresidencyStore != nil {
		if dataresidencyConfig := r.dataresidencyStore.Load(); dataresidencyConfig != nil {
			if dataresidencyConfig.DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig) {
				logging.FromContext(ctx).Debug("Updated Topic Config AllowedPersistenceRegions for Broker", zap.Any("topicConfig", *topicConfig))
			}
		}
	}
	topic, err := pubsubReconciler.ReconcileTopic(ctx, topicID, topicConfig, b, &b.Status)
	if err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.TopicID = topic.ID()

	// Check if PullSub exists, and if not, create it.
	subID := resources.GenerateDecouplingSubscriptionName(b)
	subConfig := pubsub.SubscriptionConfig{
		Topic:  topic,
		Labels: labels,
		//TODO(grantr): configure these settings?
		// AckDeadline
		// RetentionDuration
	}
	if _, err := pubsubReconciler.ReconcileSubscription(ctx, subID, subConfig, b, &b.Status); err != nil {
		return err
	}

	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.SubscriptionID = sub.ID()

	return nil
}

func (r *Reconciler) deleteDecouplingTopicAndSubscription(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling topic")

	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectIDOrDefault(r.projectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		b.Status.MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %v", err)
		b.Status.MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, b)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := resources.GenerateDecouplingTopicName(b)
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, b, &b.Status))
	subID := resources.GenerateDecouplingSubscriptionName(b)
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, b, &b.Status))

	return err
}

// createPubsubClientFn is a function for pubsub client creation. Changed in testing only.
var createPubsubClientFn reconcilerutilspubsub.CreateFn = pubsub.NewClient

// getClientOrCreateNew Return the pubsubCient if it is valid, otherwise it tries to create a new client
// and register it for later usage.
func (r *Reconciler) getClientOrCreateNew(ctx context.Context, projectID string, b *brokerv1beta1.Broker) (*pubsub.Client, error) {
	if r.pubsubClient != nil {
		return r.pubsubClient, nil
	}
	client, err := createPubsubClientFn(ctx, projectID)
	if err != nil {
		b.Status.MarkTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		b.Status.MarkSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		return nil, err
	}
	// Register the client for next run
	r.pubsubClient = client
	return client, nil
}
