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

package broker

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/trigger"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	"github.com/google/knative-gcp/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	// Name of the corev1.Events emitted from the Trigger reconciliation process.
	triggerReconciled         = "TriggerReconciled"
	triggerFinalized          = "TriggerFinalized"
	triggerReadinessChanged   = "TriggerReadinessChanged"
	triggerReconcileFailed    = "TriggerReconcileFailed"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
)

type TriggerReconciler struct {
	*reconciler.Base

	// Dynamic tracker to track KResources. It tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. It tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// CreateClientFn is the function used to create the Pub/Sub client that interacts with Pub/Sub.
	// This is needed so that we can inject a mock client for UTs purposes.
	CreateClientFn gpubsub.CreateFn

	// projectID is used as the GCP project ID when present, skipping the
	// metadata server check. Used by tests.
	projectID string
}

// Check that TriggerReconciler implements Interface
var _ triggerreconciler.Interface = (*TriggerReconciler)(nil)
var _ triggerreconciler.Finalizer = (*TriggerReconciler)(nil)

func (r *TriggerReconciler) ReconcileKind(ctx context.Context, t *brokerv1beta1.Trigger) pkgreconciler.Event {
	b := brokerFromContext(ctx)
	if b == nil {
		// Assume the Broker has been deleted or doesn't exist yet. Create a
		// Broker object with the expected name and namespace to
		// reconcile with.
		// TODO move this to resources package?
		b = &brokerv1beta1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.Spec.Broker,
				Namespace: t.Namespace,
			},
			//TODO is this needed?
			Status: brokerv1beta1.BrokerStatus{},
		}
		b.Status.InitializeConditions()
		//return fmt.Errorf("Couldn't fetch Broker from context")
	}
	t.Status.InitializeConditions()

	if b.DeletionTimestamp != nil || b.UID == "" {
		t.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist", t.Spec.Broker)
	} else {
		t.Status.PropagateBrokerStatus(&b.Status)
	}

	if err := r.resolveSubscriber(ctx, t, b); err != nil {
		return err
	}

	if err := r.reconcileRetryTopicAndSubscription(ctx, t); err != nil {
		return err
	}

	if err := r.checkDependencyAnnotation(ctx, t, b); err != nil {
		return err
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled: \"%s/%s\"", t.Namespace, t.Name)
}

func (r *TriggerReconciler) FinalizeKind(ctx context.Context, t *brokerv1beta1.Trigger) pkgreconciler.Event {
	if err := r.deleteRetryTopicAndSubscription(ctx, t); err != nil {
		return err
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, triggerFinalized, "Trigger finalized: \"%s/%s\"", t.Namespace, t.Name)

}

func (r *TriggerReconciler) resolveSubscriber(ctx context.Context, t *brokerv1beta1.Trigger, b *brokerv1beta1.Broker) error {
	if t.Spec.Subscriber.Ref != nil {
		// To call URIFromDestination(dest apisv1alpha1.Destination, parent interface{}), dest.Ref must have a Namespace
		// We will use the Namespace of Trigger as the Namespace of dest.Ref
		t.Spec.Subscriber.Ref.Namespace = t.GetNamespace()
	}

	//TODO only do this when the broker exists? It works without a UID
	subscriberURI, err := r.uriResolver.URIFromDestinationV1(t.Spec.Subscriber, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		t.Status.MarkSubscriberResolvedFailed("Unable to get the Subscriber's URI", "%v", err)
		t.Status.SubscriberURI = nil
		return err
	}
	t.Status.SubscriberURI = subscriberURI
	t.Status.MarkSubscriberResolvedSucceeded()

	return nil
}

func (r *TriggerReconciler) reconcileRetryTopicAndSubscription(ctx context.Context, trig *brokerv1beta1.Trigger) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling retry topic")
	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectID(r.projectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		return err
	}
	// Set the projectID in the status.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.ProjectID = projectID

	client, err := r.CreateClientFn(ctx, projectID)
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	defer client.Close()

	// Check if topic exists, and if not, create it.
	topicID := resources.GenerateRetryTopicName(trig)
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
				"resource":  "triggers",
				"namespace": trig.Namespace,
				"name":      trig.Name,
				//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
			},
		}
		// Create a new topic.
		logger.Debug("Creating topic with cfg", zap.String("id", topicID), zap.Any("cfg", topicConfig))
		topic, err = client.CreateTopicWithConfig(ctx, topicID, topicConfig)
		if err != nil {
			logger.Error("Failed to create Pub/Sub topic", zap.Error(err))
			trig.Status.MarkTopicFailed("CreationFailed", "Topic creation failed: %w", err)
			return err
		}
		logger.Info("Created PubSub topic", zap.String("name", topic.ID()))
		r.Recorder.Eventf(trig, corev1.EventTypeNormal, topicCreated, "Created PubSub topic %q", topic.ID())
	}

	trig.Status.MarkTopicReady()
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.TopicID = topic.ID()

	// Check if PullSub exists, and if not, create it.
	subID := resources.GenerateRetrySubscriptionName(trig)
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
				"resource":  "triggers",
				"namespace": trig.Namespace,
				"name":      trig.Name,
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
			trig.Status.MarkSubscriptionFailed("CreationFailed", "Subscription creation failed: %w", err)
			return err
		}
		logger.Info("Created PubSub subscription", zap.String("name", sub.ID()))
		r.Recorder.Eventf(trig, corev1.EventTypeNormal, subCreated, "Created PubSub subscription %q", sub.ID())
	}
	//TODO update the subscription's config if needed.

	trig.Status.MarkSubscriptionReady()
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.SubscriptionID = sub.ID()

	return nil
}

func (r *TriggerReconciler) deleteRetryTopicAndSubscription(ctx context.Context, trig *brokerv1beta1.Trigger) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting retry topic")

	// get ProjectID from metadata
	//TODO get from context
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
	topicID := resources.GenerateRetryTopicName(trig)
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
		r.Recorder.Eventf(trig, corev1.EventTypeNormal, topicDeleted, "Deleted PubSub topic %q", topic.ID())
	}

	// Delete pull subscription if it exists.
	// TODO could alternately set expiration policy to make pubsub delete it after some idle time.
	// https://cloud.google.com/pubsub/docs/admin#deleting_a_topic
	subID := resources.GenerateRetrySubscriptionName(trig)
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
		r.Recorder.Eventf(trig, corev1.EventTypeNormal, subDeleted, "Deleted PubSub subscription %q", sub.ID())
	}

	return nil
}

func (r *TriggerReconciler) checkDependencyAnnotation(ctx context.Context, t *brokerv1beta1.Trigger, b *brokerv1beta1.Broker) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[v1alpha1.DependencyAnnotation]; ok {
		dependencyObjRef, err := v1alpha1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		//TODO only do this when the broker exists? It works without a UID
		trackKResource := r.kresourceTracker.TrackInNamespace(b)
		// Trigger and its dependent source are in the same namespace, we already did the validation in the webhook.
		if err := trackKResource(dependencyObjRef); err != nil {
			return fmt.Errorf("tracking dependency: %v", err)
		}
		if err := r.propagateDependencyReadiness(ctx, t, dependencyObjRef); err != nil {
			return fmt.Errorf("propagating dependency readiness: %v", err)
		}
	} else {
		t.Status.MarkDependencySucceeded()
	}
	return nil
}

func (r *TriggerReconciler) propagateDependencyReadiness(ctx context.Context, t *brokerv1beta1.Trigger, dependencyObjRef corev1.ObjectReference) error {
	lister, err := r.kresourceTracker.ListerFor(dependencyObjRef)
	if err != nil {
		t.Status.MarkDependencyUnknown("ListerDoesNotExist", "Failed to retrieve lister: %v", err)
		return fmt.Errorf("retrieving lister: %v", err)
	}
	dependencyObj, err := lister.ByNamespace(t.GetNamespace()).Get(dependencyObjRef.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.Status.MarkDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: %v", err)
		} else {
			t.Status.MarkDependencyUnknown("DependencyGetFailed", "Failed to get dependency: %v", err)
		}
		return fmt.Errorf("getting the dependency: %v", err)
	}
	dependency := dependencyObj.(*duckv1.KResource)

	// The dependency hasn't yet reconciled our latest changes to
	// its desired state, so its conditions are outdated.
	if dependency.GetGeneration() != dependency.Status.ObservedGeneration {
		logging.FromContext(ctx).Info("The ObjectMeta Generation of dependency is not equal to the observedGeneration of status",
			zap.Any("objectMetaGeneration", dependency.GetGeneration()),
			zap.Any("statusObservedGeneration", dependency.Status.ObservedGeneration))
		t.Status.MarkDependencyUnknown("GenerationNotEqual", "The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", dependency.GetGeneration(), dependency.Status.ObservedGeneration)
		return nil
	}
	t.Status.PropagateDependencyStatus(dependency)
	return nil
}

type brokerKey struct{}

func brokerFromContext(ctx context.Context) *brokerv1beta1.Broker {
	untyped := ctx.Value(brokerKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(*brokerv1beta1.Broker)
}

func contextWithBroker(ctx context.Context, b *brokerv1beta1.Broker) context.Context {
	return context.WithValue(ctx, brokerKey{}, b)
}
