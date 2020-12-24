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

package gcpcelladdressable

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/knative-gcp/pkg/apis/duck"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/knative-gcp/pkg/broker/config"

	"github.com/rickb777/date/period"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/logging"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"

	"cloud.google.com/go/pubsub"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	// Default maximum backoff duration used in the backoff retry policy for
	// pubsub subscriptions. 600 seconds is the longest supported time.
	defaultMaximumBackoff = 600 * time.Second
)

// Reconciler implements controller.Reconciler for Trigger resources.
type TargetReconciler struct {
	ProjectID string

	// pubsubClient is used as the Pubsub client when present.
	PubsubClient *pubsub.Client

	DataresidencyStore *dataresidency.Store
}

type GCPCellDeletingTargetable interface {
	Object() runtime.Object
	StatusUpdater() reconcilerutilspubsub.StatusUpdater
	GetTopicID() string
	GetSubscriptionName() string
}

type GCPCellTargetable interface {
	GCPCellDeletingTargetable
	GetLabels() map[string]string
	DeliverySpec() *eventingduckv1beta1.DeliverySpec
	SetStatusProjectID(projectID string)
}

var _ GCPCellTargetable = (*gcpCellTargetableForTrigger)(nil)

type gcpCellTargetableForTrigger struct {
	trigger      *brokerv1beta1.Trigger
	deliverySpec *eventingduckv1beta1.DeliverySpec
}

func GCPCellTargetableFromTrigger(t *brokerv1beta1.Trigger, deliverySpec *eventingduckv1beta1.DeliverySpec) GCPCellTargetable {
	return &gcpCellTargetableForTrigger{
		trigger:      t,
		deliverySpec: deliverySpec,
	}
}

func (t gcpCellTargetableForTrigger) Key() config.GCPCellAddressableKey {
	//return config.KeyFromTrigger(t.trigger)
	panic("implement me")
}

func (t gcpCellTargetableForTrigger) Object() runtime.Object {
	return t.trigger
}

func (t gcpCellTargetableForTrigger) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &t.trigger.Status
}

func (t gcpCellTargetableForTrigger) GetLabels() map[string]string {
	return map[string]string{
		"resource":  "triggers",
		"namespace": t.trigger.Namespace,
		"name":      t.trigger.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (t gcpCellTargetableForTrigger) GetTopicID() string {
	return resources.GenerateRetryTopicName(t.trigger)
}

func (t gcpCellTargetableForTrigger) GetSubscriptionName() string {
	return resources.GenerateRetrySubscriptionName(t.trigger)
}

func (t gcpCellTargetableForTrigger) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	return t.deliverySpec
}

func (t gcpCellTargetableForTrigger) SetStatusProjectID(projectID string) {
	// TODO uncomment when eventing webhook allows this
	// t.trigger.Status.ProjectID = projectID
}

var _ GCPCellTargetable = (*gcpCellTargetableForSubscriberSpec)(nil)

type gcpCellTargetableForSubscriberSpec struct {
	subscriberSpec *eventingduckv1beta1.SubscriberSpec
	channel        *v1beta1.Channel
	status         *SubscriberStatus
}

func GCPCellTargetableFromSubscriberSpec(channel *v1beta1.Channel, subscribeSpec eventingduckv1beta1.SubscriberSpec) (GCPCellTargetable, *SubscriberStatus) {
	status := &SubscriberStatus{}
	return &gcpCellTargetableForSubscriberSpec{
		subscriberSpec: &subscribeSpec,
		channel:        channel,
		status:         status,
	}, status
}

func (s gcpCellTargetableForSubscriberSpec) Object() runtime.Object {
	return s.channel
}

func (s gcpCellTargetableForSubscriberSpec) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return s.status
}

func (s gcpCellTargetableForSubscriberSpec) GetLabels() map[string]string {
	labels := map[string]string{
		"resource":          "subscriptions",
		"channel-namespace": s.channel.Namespace,
		"channel-name":      s.channel.Name,
		"sub-uid":           string(s.subscriberSpec.UID),
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
	if clusterName := s.channel.GetAnnotations()[duck.ClusterNameAnnotation]; clusterName != "" {
		labels[duck.ClusterNameAnnotation] = clusterName
	}
	return labels
}

func (s gcpCellTargetableForSubscriberSpec) GetTopicID() string {
	return resources.GenerateSubscriberRetryTopicName(s.channel, s.subscriberSpec.UID)
}

func (s gcpCellTargetableForSubscriberSpec) GetSubscriptionName() string {
	return resources.GenerateSubscriberRetrySubscriptionName(s.channel, s.subscriberSpec.UID)
}

func (s gcpCellTargetableForSubscriberSpec) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	return s.subscriberSpec.Delivery
}

func (s gcpCellTargetableForSubscriberSpec) SetStatusProjectID(_ string) {
	// ProjectID is stored on the Channel's status, not each subscriber's, so this is a noop.
}

func (r *TargetReconciler) ReconcileRetryTopicAndSubscription(ctx context.Context, recorder record.EventRecorder, t GCPCellTargetable) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling retry topic")
	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		t.StatusUpdater().MarkTopicUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		t.StatusUpdater().MarkSubscriptionUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}
	t.SetStatusProjectID(projectID)

	client, err := r.getClientOrCreateNew(ctx, projectID, t.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}

	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, recorder)

	// Check if topic exists, and if not, create it.
	topicID := t.GetTopicID()
	topicConfig := &pubsub.TopicConfig{Labels: t.GetLabels()}
	if r.DataresidencyStore != nil {
		if dataresidencyConfig := r.DataresidencyStore.Load(); dataresidencyConfig != nil {
			if dataresidencyConfig.DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig) {
				logging.FromContext(ctx).Debug("Updated Topic Config AllowedPersistenceRegions for Trigger", zap.Any("topicConfig", *topicConfig))
			}
		}
	}
	topic, err := pubsubReconciler.ReconcileTopic(ctx, topicID, topicConfig, t.Object(), t.StatusUpdater())
	if err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//trig.Status.TopicID = topic.ID()

	retryPolicy := getPubsubRetryPolicy(t.DeliverySpec())
	deadLetterPolicy := getPubsubDeadLetterPolicy(projectID, t.DeliverySpec())

	// Check if PullSub exists, and if not, create it.
	subID := t.GetSubscriptionName()
	subConfig := pubsub.SubscriptionConfig{
		Topic:            topic,
		Labels:           t.GetLabels(),
		RetryPolicy:      retryPolicy,
		DeadLetterPolicy: deadLetterPolicy,
		//TODO(grantr): configure these settings?
		// AckDeadline
		// RetentionDuration
	}
	if _, err := pubsubReconciler.ReconcileSubscription(ctx, subID, subConfig, t.Object(), t.StatusUpdater()); err != nil {
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

func (r *TargetReconciler) DeleteRetryTopicAndSubscription(ctx context.Context, recorder record.EventRecorder, t GCPCellDeletingTargetable) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting retry topic")

	// get ProjectID from metadata
	//TODO get from context
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		t.StatusUpdater().MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %v", err)
		t.StatusUpdater().MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, t.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := t.GetTopicID()
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, t.Object(), t.StatusUpdater()))
	// Delete pull subscription if it exists.
	subID := t.GetSubscriptionName()
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, t.Object(), t.StatusUpdater()))
	return err
}

// getClientOrCreateNew Return the pubsubCient if it is valid, otherwise it tries to create a new client
// and register it for later usage.
func (r *TargetReconciler) getClientOrCreateNew(ctx context.Context, projectID string, b reconcilerutilspubsub.StatusUpdater) (*pubsub.Client, error) {
	if r.PubsubClient != nil {
		return r.PubsubClient, nil
	}
	client, err := createPubsubClientFn(ctx, projectID)
	if err != nil {
		b.MarkTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		b.MarkSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		return nil, err
	}
	// Register the client for next run
	r.PubsubClient = client
	return client, nil
}

var _ reconcilerutilspubsub.StatusUpdater = (*SubscriberStatus)(nil)

type SubscriberStatus struct {
	topicStatus         corev1.ConditionStatus
	topicMessage        string
	subscriptionStatus  corev1.ConditionStatus
	subscriptionMessage string
}

func (s SubscriberStatus) MarkTopicFailed(reason, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s SubscriberStatus) MarkTopicUnknown(_, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s SubscriberStatus) MarkTopicReady() {
	s.topicStatus = corev1.ConditionTrue
	s.topicMessage = ""
}

func (s SubscriberStatus) MarkSubscriptionFailed(_, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s SubscriberStatus) MarkSubscriptionUnknown(_, format string, args ...interface{}) {
	s.subscriptionStatus = corev1.ConditionFalse
	s.subscriptionMessage = fmt.Sprintf(format, args...)
}

func (s SubscriberStatus) MarkSubscriptionReady() {
	s.subscriptionStatus = corev1.ConditionTrue
	s.subscriptionMessage = ""
}

func (s SubscriberStatus) Ready() corev1.ConditionStatus {
	if s.topicStatus == corev1.ConditionFalse || s.subscriptionStatus == corev1.ConditionFalse {
		return corev1.ConditionFalse
	}
	if s.topicStatus == corev1.ConditionUnknown || s.subscriptionStatus == corev1.ConditionUnknown {
		return corev1.ConditionUnknown
	}
	return corev1.ConditionTrue
}

func (s SubscriberStatus) Message() string {
	if s.topicMessage != "" {
		return s.topicMessage
	}
	if s.subscriptionMessage != "" {
		return s.subscriptionMessage
	}
	return ""
}

var _ GCPCellDeletingTargetable = (*gcpCellTargetableForSubscriberStatus)(nil)

type gcpCellTargetableForSubscriberStatus struct {
	subscriberStatus *eventingduckv1beta1.SubscriberStatus
	channel          *v1beta1.Channel
	status           *SubscriberStatus
}

func GCPCellTargetableFromSubscriberStatus(channel *v1beta1.Channel, subscriberStatus eventingduckv1beta1.SubscriberStatus) (GCPCellDeletingTargetable, *SubscriberStatus) {
	status := &SubscriberStatus{}
	return &gcpCellTargetableForSubscriberStatus{
		subscriberStatus: &subscriberStatus,
		channel:          channel,
		status:           status,
	}, status
}

func (s gcpCellTargetableForSubscriberStatus) Object() runtime.Object {
	return s.channel
}

func (s gcpCellTargetableForSubscriberStatus) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return s.status
}

func (s gcpCellTargetableForSubscriberStatus) GetTopicID() string {
	return resources.GenerateSubscriberRetryTopicName(s.channel, s.subscriberStatus.UID)
}

func (s gcpCellTargetableForSubscriberStatus) GetSubscriptionName() string {
	return resources.GenerateSubscriberRetrySubscriptionName(s.channel, s.subscriberStatus.UID)
}
