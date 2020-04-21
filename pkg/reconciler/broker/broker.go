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

	"cloud.google.com/go/pubsub"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	brokerlisters "github.com/google/knative-gcp/pkg/client/listers/broker/v1beta1"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	"github.com/google/knative-gcp/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerReconcileError = "BrokerReconcileError"
	brokerReconciled     = "BrokerReconciled"
	brokerFinalized      = "BrokerFinalized"
	topicCreated         = "TopicCreated"
	subCreated           = "SubscriptionCreated"
	topicDeleted         = "TopicDeleted"
	subDeleted           = "SubscriptionDeleted"

	targetsCMName = "broker-targets"

	ingressServiceName = "broker-ingress"
)

// TODO
// idea: assign broker resources to cell (configmap) in webhook based on a
// global configmap (in controller's namespace) of cell assignment rules, and
// label the broker with the assignment. Controller uses these labels to
// determine which configmap to create/update when a broker is reconciled, and
// to determine which brokers to reconcile when a configmap is updated.
// Initially, the assignment can be static.

// TODO bug: if broker is deleted first, triggers can't be reconciled
// TODO: verify that if topic is deleted from gcp it's recreated here

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	triggerLister   brokerlisters.TriggerLister
	configMapLister corev1listers.ConfigMapLister
	endpointsLister corev1listers.EndpointsLister

	// CreateClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	CreateClientFn gpubsub.CreateFn

	// Reconciles a broker's triggers
	triggerReconciler controller.Reconciler

	// Updates broker configuration
	brokerConfigUpdater BrokerConfigUpdater

	// projectID is used as the GCP project ID when present, skipping the
	// metadata server check. Used by tests.
	projectID string
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

	triggers, err := r.reconcileTriggers(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling triggers", zap.Error(err))
		return fmt.Errorf("failed to reconcile triggers: %w", err)
	}

	if err := r.reconcileTargets(ctx, b, triggers); err != nil {
		return fmt.Errorf("failed to reconcile targets config: %w", err)
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerReconciled, "Broker reconciled: \"%s/%s\"", b.Namespace, b.Name)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *brokerv1beta1.Broker) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Debug("Finalizing Broker", zap.Any("broker", b))

	// Reconcile triggers so they update their status
	// TODO is this the best way to reconcile triggers when their broker is
	// being deleted?
	if _, err := r.reconcileTriggers(ctx, b); err != nil {
		logger.Error("Problem reconciling triggers", zap.Error(err))
		return fmt.Errorf("failed to reconcile triggers: %w", err)
	}

	if err := r.deleteDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("Failed to delete Pub/Sub topic: %w", err)
	}

	// TODO should we also delete the broker from the config? Maybe better
	// to keep the targets in place if there are still triggers. But we can
	// update the status to UNKNOWN and remove address etc. Maybe need a new
	// status DELETED or a deleted timestamp that can be used to clean up later.

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerFinalized, "Broker finalized: \"%s/%s\"", b.Namespace, b.Name)

}

func (r *Reconciler) reconcileBroker(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling Broker", zap.Any("broker", b))
	b.Status.InitializeConditions()
	b.Status.ObservedGeneration = b.Generation

	// Create decoupling topic and pullsub for this broker. Ingress will push
	// to this topic and fanout will pull from the pull sub.
	if err := r.reconcileDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("Decoupling topic reconcile failed: %w", err)
	}

	//TODO in a webhook annotate the broker object with its ingress details
	// Route to shared ingress with namespace/name in the path as the broker
	// identifier.
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(ingressServiceName, system.Namespace()),
		Path:   fmt.Sprintf("/%s/%s", b.Namespace, b.Name),
	})

	// Verify the ingress service is healthy via endpoints.
	ingressEndpoints, err := r.endpointsLister.Endpoints(system.Namespace()).Get(ingressServiceName)
	if err != nil {
		logger.Error("Problem getting endpoints for ingress", zap.String("namespace", system.Namespace()), zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressAvailability(ingressEndpoints)

	return nil
}

func (r *Reconciler) reconcileDecouplingTopicAndSubscription(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling decoupling topic", zap.Any("broker", b))
	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectID(r.projectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		return err
	}
	// Set the projectID in the status.
	//TODO uncomment when eventing webhook allows this
	//b.Status.ProjectID = projectID

	client, err := r.CreateClientFn(ctx, projectID)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Check if topic exists, and if not, create it.
	topicID := resources.GenerateDecouplingTopicName(b)
	topic := client.Topic(topicID)
	exists, err := topic.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}

	if !exists {
		// TODO If this can ever change through the Broker's lifecycle, add
		// update handling
		topicConfig := &pubsub.TopicConfig{
			Labels: map[string]string{
				"resource":     "brokers",
				"broker_class": brokerv1beta1.BrokerClass,
				"namespace":    b.Namespace,
				"name":         b.Name,
				//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
			},
		}
		// Create a new topic.
		logger.Debug("Creating topic with cfg", zap.String("id", topicID), zap.Any("cfg", topicConfig))
		topic, err = client.CreateTopicWithConfig(ctx, topicID, topicConfig)
		if err != nil {
			logger.Error("Failed to create Pub/Sub topic", zap.Error(err))
			b.Status.MarkTopicFailed("CreationFailed", "Topic creation failed: %w", err)
			return err
		}
		logger.Info("Created PubSub topic", zap.String("name", topic.ID()))
		r.Recorder.Eventf(b, corev1.EventTypeNormal, topicCreated, "Created PubSub topic %q", topic.ID())
	}

	b.Status.MarkTopicReady()
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.TopicID = topic.ID()

	// Check if PullSub exists, and if not, create it.
	subID := resources.GenerateDecouplingSubscriptionName(b)
	sub := client.Subscription(subID)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub subscription exists", zap.Error(err))
		return err
	}

	if !subExists {
		// TODO If this can ever change through the Broker's lifecycle, add
		// update handling
		subConfig := gpubsub.SubscriptionConfig{
			Topic: topic,
			Labels: map[string]string{
				"resource":     "brokers",
				"broker_class": brokerv1beta1.BrokerClass,
				"namespace":    b.Namespace,
				"name":         b.Name,
				//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
			},
			//TODO(grantr): configure these settings?
			// AckDeadline
			// RetentionDuration
		}
		// Create a new subscription to the previous topic with the given name.
		logger.Debug("Creating sub with cfg", zap.String("id", subID), zap.Any("cfg", subConfig))
		sub, err = client.CreateSubscription(ctx, subID, subConfig)
		if err != nil {
			logger.Error("Failed to create subscription", zap.Error(err))
			b.Status.MarkSubscriptionFailed("CreationFailed", "Subscription creation failed: %w", err)
			return err
		}
		logger.Info("Created PubSub subscription", zap.String("name", sub.ID()))
		r.Recorder.Eventf(b, corev1.EventTypeNormal, subCreated, "Created PubSub subscription %q", sub.ID())
	}

	b.Status.MarkSubscriptionReady()
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.SubscriptionID = sub.ID()

	return nil
}

func (r *Reconciler) deleteDecouplingTopicAndSubscription(ctx context.Context, b *brokerv1beta1.Broker) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling topic")

	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectID(r.projectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		return err
	}

	client, err := r.CreateClientFn(ctx, projectID)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := resources.GenerateDecouplingTopicName(b)
	topic := client.Topic(topicID)
	exists, err := topic.Exists(ctx)
	if err != nil {
		logger.Error("Failed to verify Pub/Sub topic exists", zap.Error(err))
		return err
	}
	if exists {
		if err := topic.Delete(ctx); err != nil {
			logger.Error("Failed to delete Pub/Sub topic", zap.Error(err))
			return err
		}
		logger.Info("Deleted PubSub topic", zap.String("name", topic.ID()))
		r.Recorder.Eventf(b, corev1.EventTypeNormal, topicDeleted, "Deleted PubSub topic %q", topic.ID())
	}

	// Delete pull subscription if it exists.
	// TODO could alternately set expiration policy to make pubsub delete it after some idle time.
	// https://cloud.google.com/pubsub/docs/admin#deleting_a_topic
	subID := resources.GenerateDecouplingSubscriptionName(b)
	sub := client.Subscription(subID)
	exists, err = sub.Exists(ctx)
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
		r.Recorder.Eventf(b, corev1.EventTypeNormal, subDeleted, "Deleted PubSub subscription %q", sub.ID())
	}

	return nil
}

// reconcileTriggers reconciles the Triggers that are pointed to this broker
func (r *Reconciler) reconcileTriggers(ctx context.Context, b *brokerv1beta1.Broker) ([]*brokerv1beta1.Trigger, error) {
	// Filter by `eventing.knative.dev/broker: <name>` here
	// to get only the triggers for this broker. The trigger webhook will
	// ensure that triggers are always labeled with their broker name.
	allTriggers, err := r.triggerLister.Triggers(b.Namespace).List(
		labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: b.Name}))
	if err != nil {
		return nil, err
	}

	ctx = contextWithBroker(ctx, b)
	var triggers []*brokerv1beta1.Trigger
	for _, t := range allTriggers {
		if t.Spec.Broker == b.Name {
			logger := logging.FromContext(ctx).With(zap.String("trigger", t.Name), zap.String("broker", b.Name))
			ctx = logging.WithLogger(ctx, logger)

			if tKey, err := cache.MetaNamespaceKeyFunc(t); err == nil {
				err = r.triggerReconciler.Reconcile(ctx, tKey)
			}
			triggers = append(triggers, t)
		}
	}
	return triggers, err
}

// reconcileTriggers reconciles the Triggers that are pointed to this broker
func (r *Reconciler) reconcileTargets(ctx context.Context, b *brokerv1beta1.Broker, triggers []*brokerv1beta1.Trigger) error {
	brokerConfig := &config.Broker{
		Id:        string(b.UID),
		Name:      b.Name,
		Namespace: b.Namespace,
		Address:   b.Status.Address.URL.String(),
		DecoupleQueue: &config.Queue{
			Topic:        resources.GenerateDecouplingTopicName(b),
			Subscription: resources.GenerateDecouplingSubscriptionName(b),
		},
		Targets: make(map[string]*config.Target),
	}
	if b.Status.IsReady() {
		brokerConfig.State = config.State_READY
	} else {
		brokerConfig.State = config.State_UNKNOWN
	}

	for _, t := range triggers {
		if t.DeletionTimestamp == nil {
			brokerConfig.Targets[t.Name] = triggerTargetConfig(b, t)
		}
	}

	return r.brokerConfigUpdater.UpdateBrokerConfig(ctx, brokerConfig)
}

func triggerTargetConfig(b *brokerv1beta1.Broker, t *brokerv1beta1.Trigger) *config.Target {
	target := &config.Target{
		Id:        string(t.UID),
		Name:      t.Name,
		Namespace: t.Namespace,
		Broker:    b.Name,
		Address:   t.Status.SubscriberURI.String(),
		RetryQueue: &config.Queue{
			Topic:        resources.GenerateRetryTopicName(t),
			Subscription: resources.GenerateRetrySubscriptionName(t),
		},
	}
	if t.Spec.Filter != nil && t.Spec.Filter.Attributes != nil {
		target.FilterAttributes = t.Spec.Filter.Attributes
	}
	if t.Status.IsReady() {
		target.State = config.State_READY
	} else {
		target.State = config.State_UNKNOWN
	}
	return target
}
