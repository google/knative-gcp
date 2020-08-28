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

package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/rickb777/date/period"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"cloud.google.com/go/pubsub"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/trigger"
	brokerlisters "github.com/google/knative-gcp/pkg/client/listers/broker/v1beta1"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
)

const (
	// Name of the corev1.Events emitted from the Trigger reconciliation process.
	triggerReconciled = "TriggerReconciled"
	triggerFinalized  = "TriggerFinalized"

	// Default maximum backoff duration used in the backoff retry policy for
	// pubsub subscriptions. 600 seconds is the longest supported time.
	defaultMaximumBackoff = 600 * time.Second
)

var (
	// Default backoff policy settings. Should normally be configured through the
	// br-delivery ConfigMap, but these values serve in case the intended
	// defaulting fails.
	defaultBackoffDelay  = "PT1S"
	defaultBackoffPolicy = eventingduckv1beta1.BackoffPolicyExponential
)

// Reconciler implements controller.Reconciler for Trigger resources.
type Reconciler struct {
	*reconciler.Base

	brokerLister brokerlisters.BrokerLister

	// Dynamic tracker to track KResources. It tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. It tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	projectID string

	// pubsubClient is used as the Pubsub client when present.
	pubsubClient *pubsub.Client
}

// Check that TriggerReconciler implements Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)
var _ triggerreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, t *brokerv1beta1.Trigger) pkgreconciler.Event {
	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)

	if err != nil && !apierrs.IsNotFound(err) {
		// Unknown error. genreconciler will record an `InternalError` event and keep retrying.
		return err
	}

	// If the broker has been or is being deleted, we clean up resources created by this controller
	// for the given trigger.
	if apierrs.IsNotFound(err) || !b.GetDeletionTimestamp().IsZero() {
		return r.FinalizeKind(ctx, t)
	}

	if !filterBroker(b) {
		// Call Finalizer anyway in case the Trigger still holds GCP Broker related resources.
		// If a Trigger used to point to a GCP Broker but now has a Broker with a different brokerclass,
		// we should clean up resources related to GCP Broker.
		return r.FinalizeKind(ctx, t)
	}

	return r.reconcile(ctx, t, b)
}

// reconciles the Trigger given that its Broker exists and is not being deleted.
func (r *Reconciler) reconcile(ctx context.Context, t *brokerv1beta1.Trigger, b *brokerv1beta1.Broker) pkgreconciler.Event {
	t.Status.InitializeConditions()
	t.Status.PropagateBrokerStatus(&b.Status)

	if err := r.resolveSubscriber(ctx, t, b); err != nil {
		return err
	}

	if b.Spec.Delivery == nil {
		b.SetDefaults(ctx)
	}
	if err := r.reconcileRetryTopicAndSubscription(ctx, t, b.Spec.Delivery); err != nil {
		return err
	}

	if err := r.checkDependencyAnnotation(ctx, t); err != nil {
		return err
	}

	return pkgreconciler.NewEvent(corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled: \"%s/%s\"", t.Namespace, t.Name)
}

// FinalizeKind frees GCP Broker related resources for this Trigger if applicable. It's called when:
// 1) the Trigger is being deleted;
// 2) the Broker of this Trigger is deleted;
// 3) the Broker of this Trigger is updated with one that is not a GCP broker.
func (r *Reconciler) FinalizeKind(ctx context.Context, t *brokerv1beta1.Trigger) pkgreconciler.Event {
	// Don't care if the Trigger doesn't have the GCP Broker specific finalizer string.
	// Right now all triggers have the finalizer because genreconciler automatically adds it.
	// TODO(https://github.com/knative/pkg/issues/1149) Add a FilterKind to genreconciler so it will
	// skip a trigger if it's not pointed to a gcp broker and doesn't have googlecloud finalizer string.
	if !hasGCPBrokerFinalizer(t) {
		return nil
	}
	if err := r.deleteRetryTopicAndSubscription(ctx, t); err != nil {
		return err
	}
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, triggerFinalized, "Trigger finalized: \"%s/%s\"", t.Namespace, t.Name)
}

func (r *Reconciler) resolveSubscriber(ctx context.Context, t *brokerv1beta1.Trigger, b *brokerv1beta1.Broker) error {
	if t.Spec.Subscriber.Ref != nil {
		// To call URIFromDestination(dest apisv1alpha1.Destination, parent interface{}), dest.Ref must have a Namespace
		// We will use the Namespace of Trigger as the Namespace of dest.Ref
		t.Spec.Subscriber.Ref.Namespace = t.GetNamespace()
	}

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

// hasGCPBrokerFinalizer checks if the Trigger object has a finalizer matching the one added by this controller.
func hasGCPBrokerFinalizer(t *brokerv1beta1.Trigger) bool {
	for _, f := range t.Finalizers {
		if f == finalizerName {
			return true
		}
	}
	return false
}

func (r *Reconciler) reconcileRetryTopicAndSubscription(ctx context.Context, trig *brokerv1beta1.Trigger, deliverySpec *eventingduckv1beta1.DeliverySpec) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling retry topic")
	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectID(r.projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		trig.Status.MarkTopicUnknown("ProjectIdNotFound", "Failed to find project id: %w", err)
		trig.Status.MarkSubscriptionUnknown("ProjectIdNotFound", "Failed to find project id: %w", err)
		return err
	}
	// Set the projectID in the status.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.ProjectID = projectID

	client := r.pubsubClient
	if client == nil {
		client, err := pubsub.NewClient(ctx, projectID)
		if err != nil {
			logger.Error("Failed to create Pub/Sub client", zap.Error(err))
			trig.Status.MarkTopicUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: %w", err)
			trig.Status.MarkSubscriptionUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: %w", err)
			return err
		}
		defer client.Close()
	}

	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	labels := map[string]string{
		"resource":  "triggers",
		"namespace": trig.Namespace,
		"name":      trig.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}

	// Check if topic exists, and if not, create it.
	topicID := resources.GenerateRetryTopicName(trig)
	topicConfig := &pubsub.TopicConfig{Labels: labels}
	topic, err := pubsubReconciler.ReconcileTopic(ctx, topicID, topicConfig, trig, &trig.Status)
	if err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.TopicID = topic.ID()

	retryPolicy := getPubsubRetryPolicy(deliverySpec)
	deadLetterPolicy := getPubsubDeadLetterPolicy(projectID, deliverySpec)

	// Check if PullSub exists, and if not, create it.
	subID := resources.GenerateRetrySubscriptionName(trig)
	subConfig := pubsub.SubscriptionConfig{
		Topic:            topic,
		Labels:           labels,
		RetryPolicy:      retryPolicy,
		DeadLetterPolicy: deadLetterPolicy,
		//TODO(grantr): configure these settings?
		// AckDeadline
		// RetentionDuration
	}
	if _, err := pubsubReconciler.ReconcileSubscription(ctx, subID, subConfig, trig, &trig.Status); err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.SubscriptionID = sub.ID()

	return nil
}

// getPubsubRetryPolicy gets the eventing retry policy from the Broker delivery
// spec and translates it to a pubsub retry policy.
func getPubsubRetryPolicy(spec *eventingduckv1beta1.DeliverySpec) *pubsub.RetryPolicy {
	// The Broker delivery spec is translated to a pubsub retry policy in the
	// manner defined in the following post:
	// https://github.com/google/knative-gcp/issues/1392#issuecomment-655617873
	p, _ := period.Parse(*spec.BackoffDelay)
	minimumBackoff, _ := p.Duration()
	var maximumBackoff time.Duration
	switch *spec.BackoffPolicy {
	case eventingduckv1beta1.BackoffPolicyLinear:
		maximumBackoff = minimumBackoff
	case eventingduckv1beta1.BackoffPolicyExponential:
		maximumBackoff = defaultMaximumBackoff
	}
	return &pubsub.RetryPolicy{
		MinimumBackoff: minimumBackoff,
		MaximumBackoff: maximumBackoff,
	}
}

// getPubsubDeadLetterPolicy gets the eventing dead letter policy from the
// Broker delivery spec and translates it to a pubsub dead letter policy.
func getPubsubDeadLetterPolicy(projectID string, spec *eventingduckv1beta1.DeliverySpec) *pubsub.DeadLetterPolicy {
	if spec.DeadLetterSink == nil {
		return nil
	}
	// Translate to the pubsub dead letter policy format.
	return &pubsub.DeadLetterPolicy{
		MaxDeliveryAttempts: int(*spec.Retry),
		DeadLetterTopic:     fmt.Sprintf("projects/%s/topics/%s", projectID, spec.DeadLetterSink.URI.Host),
	}
}

func (r *Reconciler) deleteRetryTopicAndSubscription(ctx context.Context, trig *brokerv1beta1.Trigger) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting retry topic")

	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectID(r.projectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		trig.Status.MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %w", err)
		trig.Status.MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %w", err)
		return err
	}

	client := r.pubsubClient
	if client == nil {
		client, err := pubsub.NewClient(ctx, projectID)
		if err != nil {
			logger.Error("Failed to create Pub/Sub client", zap.Error(err))
			trig.Status.MarkTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: %w", err)
			trig.Status.MarkSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: %w", err)
			return err
		}
		defer client.Close()
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := resources.GenerateRetryTopicName(trig)
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, trig, &trig.Status))
	// Delete pull subscription if it exists.
	subID := resources.GenerateRetrySubscriptionName(trig)
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, trig, &trig.Status))
	return err
}

func (r *Reconciler) checkDependencyAnnotation(ctx context.Context, t *brokerv1beta1.Trigger) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[v1beta1.DependencyAnnotation]; ok {
		dependencyObjRef, err := v1beta1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		trackKResource := r.kresourceTracker.TrackInNamespace(t)
		// Trigger and its dependent source are in the same namespace, we already did the validation in the webhook.
		if err := trackKResource(dependencyObjRef); err != nil {
			t.Status.MarkDependencyUnknown("TrackingError", "Unable to track dependency: %v", err)
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

func (r *Reconciler) propagateDependencyReadiness(ctx context.Context, t *brokerv1beta1.Trigger, dependencyObjRef corev1.ObjectReference) error {
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
