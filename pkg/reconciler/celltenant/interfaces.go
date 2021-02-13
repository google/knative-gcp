/*
Copyright 2021 Google LLC

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

package celltenant

import (
	"fmt"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/duck"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config"
	brokerresources "github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	channelresources "github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
)

// Target is the interface that the TargetReconciler uses for reconciliation.
type Target interface {
	Object() runtime.Object
	StatusUpdater() reconcilerutilspubsub.StatusUpdater
	GetTopicID() string
	GetSubscriptionName() string
	GetLabels() map[string]string
	DeliverySpec() *eventingduckv1beta1.DeliverySpec
	SetStatusProjectID(projectID string)
}

var _ Target = (*targetForTrigger)(nil)

type targetForTrigger struct {
	trigger      *brokerv1beta1.Trigger
	deliverySpec *eventingduckv1beta1.DeliverySpec
}

// TargetFromTrigger creates a Target for the given Trigger and associated
// Broker's deliverySpec.
func TargetFromTrigger(t *brokerv1beta1.Trigger, deliverySpec *eventingduckv1beta1.DeliverySpec) Target {
	return &targetForTrigger{
		trigger:      t,
		deliverySpec: deliverySpec,
	}
}

func (t *targetForTrigger) Object() runtime.Object {
	return t.trigger
}

func (t *targetForTrigger) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &t.trigger.Status
}

func (t *targetForTrigger) GetLabels() map[string]string {
	return map[string]string{
		"resource":  "triggers",
		"namespace": t.trigger.Namespace,
		"name":      t.trigger.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (t *targetForTrigger) GetTopicID() string {
	return brokerresources.GenerateRetryTopicName(t.trigger)
}

func (t *targetForTrigger) GetSubscriptionName() string {
	return brokerresources.GenerateRetrySubscriptionName(t.trigger)
}

func (t *targetForTrigger) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	return t.deliverySpec
}

func (t *targetForTrigger) SetStatusProjectID(_ string) {
	// TODO uncomment when eventing webhook allows this
	// t.trigger.Status.ProjectID = projectID
}

var _ Target = (*targetForSubscriberSpec)(nil)

type targetForSubscriberSpec struct {
	subscriberSpec *eventingduckv1beta1.SubscriberSpec
	channel        *v1beta1.Channel
	status         *SubscriberStatus
}

func TargetFromSubscriberSpec(channel *v1beta1.Channel, subscribeSpec eventingduckv1beta1.SubscriberSpec) (Target, *SubscriberStatus) {
	status := &SubscriberStatus{}
	return &targetForSubscriberSpec{
		subscriberSpec: &subscribeSpec,
		channel:        channel,
		status:         status,
	}, status
}

func (s *targetForSubscriberSpec) Object() runtime.Object {
	return s.channel
}

func (s *targetForSubscriberSpec) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return s.status
}

func (s *targetForSubscriberSpec) GetLabels() map[string]string {
	return getLabelsForChannel(s.channel, s.subscriberSpec.UID)
}

func getLabelsForChannel(channel *v1beta1.Channel, subscriberUID types.UID) map[string]string {
	labels := map[string]string{
		"resource":          "subscriptions",
		"channel-namespace": channel.Namespace,
		"channel-name":      channel.Name,
		"sub-uid":           string(subscriberUID),
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
	if clusterName := channel.GetAnnotations()[duck.ClusterNameAnnotation]; clusterName != "" {
		labels[duck.ClusterNameAnnotation] = clusterName
	}
	return labels
}

func (s *targetForSubscriberSpec) GetTopicID() string {
	return channelresources.GenerateSubscriberRetryTopicName(s.channel, s.subscriberSpec.UID)
}

func (s *targetForSubscriberSpec) GetSubscriptionName() string {
	return channelresources.GenerateSubscriberRetrySubscriptionName(s.channel, s.subscriberSpec.UID)
}

func (s *targetForSubscriberSpec) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	return s.subscriberSpec.Delivery
}

func (s *targetForSubscriberSpec) SetStatusProjectID(_ string) {
	// ProjectID is stored on the Channel's status, not each subscriber's, so this is a noop.
}

var _ Target = (*targetForSubscriberStatus)(nil)

type targetForSubscriberStatus struct {
	subscriberStatus *eventingduckv1beta1.SubscriberStatus
	channel          *v1beta1.Channel
	status           *SubscriberStatus
}

func (s *targetForSubscriberStatus) GetLabels() map[string]string {
	return getLabelsForChannel(s.channel, s.subscriberStatus.UID)
}

func (s *targetForSubscriberStatus) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	// SubscriberStatus does not contain the DeliverySpec. This should only be used for deleting a
	// Target, not creating one, so this shouldn't be needed.
	return nil
}

func (s *targetForSubscriberStatus) SetStatusProjectID(_ string) {
	// ProjectID is stored on the Channel's status, not each subscriber's, so this is a noop.
}

func TargetFromSubscriberStatus(channel *v1beta1.Channel, subscriberStatus eventingduckv1beta1.SubscriberStatus) (Target, *SubscriberStatus) {
	status := &SubscriberStatus{}
	return &targetForSubscriberStatus{
		subscriberStatus: &subscriberStatus,
		channel:          channel,
		status:           status,
	}, status
}

func (s *targetForSubscriberStatus) Object() runtime.Object {
	return s.channel
}

func (s *targetForSubscriberStatus) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return s.status
}

func (s *targetForSubscriberStatus) GetTopicID() string {
	return channelresources.GenerateSubscriberRetryTopicName(s.channel, s.subscriberStatus.UID)
}

func (s *targetForSubscriberStatus) GetSubscriptionName() string {
	return channelresources.GenerateSubscriberRetrySubscriptionName(s.channel, s.subscriberStatus.UID)
}

var _ reconcilerutilspubsub.StatusUpdater = (*SubscriberStatus)(nil)

type SubscriberStatus struct {
	topicStatus         corev1.ConditionStatus
	topicMessage        string
	subscriptionStatus  corev1.ConditionStatus
	subscriptionMessage string
}

func (s *SubscriberStatus) MarkTopicFailed(_, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s *SubscriberStatus) MarkTopicUnknown(_, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s *SubscriberStatus) MarkTopicReady() {
	s.topicStatus = corev1.ConditionTrue
	s.topicMessage = ""
}

func (s *SubscriberStatus) MarkSubscriptionFailed(_, format string, args ...interface{}) {
	s.topicStatus = corev1.ConditionFalse
	s.topicMessage = fmt.Sprintf(format, args...)
}

func (s *SubscriberStatus) MarkSubscriptionUnknown(_, format string, args ...interface{}) {
	s.subscriptionStatus = corev1.ConditionFalse
	s.subscriptionMessage = fmt.Sprintf(format, args...)
}

func (s *SubscriberStatus) MarkSubscriptionReady(_ string) {
	s.subscriptionStatus = corev1.ConditionTrue
	s.subscriptionMessage = ""
}

func (s *SubscriberStatus) Ready() corev1.ConditionStatus {
	if s.topicStatus == corev1.ConditionFalse || s.subscriptionStatus == corev1.ConditionFalse {
		return corev1.ConditionFalse
	}
	if s.topicStatus == corev1.ConditionUnknown || s.subscriptionStatus == corev1.ConditionUnknown {
		return corev1.ConditionUnknown
	}
	return corev1.ConditionTrue
}

func (s *SubscriberStatus) Message() string {
	if s.topicMessage != "" {
		return s.topicMessage
	}
	if s.subscriptionMessage != "" {
		return s.subscriptionMessage
	}
	return ""
}

// Statusable is the interface used by the Reconciler.
type Statusable interface {
	Key() *config.CellTenantKey
	MarkBrokerCellReady()
	MarkBrokerCellUnknown(reason, format string, args ...interface{})
	MarkBrokerCellFailed(reason, format string, args ...interface{})
	SetAddress(*apis.URL)
	Object() runtime.Object
	StatusUpdater() reconcilerutilspubsub.StatusUpdater
	GetLabels() map[string]string
	GetTopicID() string
	GetSubscriptionName() string
}

var _ Statusable = (*statusableForBroker)(nil)

type statusableForBroker struct {
	broker *brokerv1beta1.Broker
}

func StatusableFromBroker(b *brokerv1beta1.Broker) Statusable {
	return &statusableForBroker{
		broker: b,
	}
}

func (b *statusableForBroker) Key() *config.CellTenantKey {
	return config.KeyFromBroker(b.broker)
}

func (b *statusableForBroker) MarkBrokerCellReady() {
	b.broker.Status.MarkBrokerCellReady()
}

func (b *statusableForBroker) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellUnknown(reason, format, args...)
}

func (b *statusableForBroker) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellFailed(reason, format, args...)
}

func (b *statusableForBroker) SetAddress(url *apis.URL) {
	b.broker.Status.SetAddress(url)
}

func (b *statusableForBroker) Object() runtime.Object {
	return b.broker
}

func (b *statusableForBroker) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &b.broker.Status
}

func (b *statusableForBroker) GetLabels() map[string]string {
	return map[string]string{
		"resource":     "brokers",
		"broker_class": brokerv1beta1.BrokerClass,
		"namespace":    b.broker.Namespace,
		"name":         b.broker.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (b *statusableForBroker) GetTopicID() string {
	return brokerresources.GenerateDecouplingTopicName(b.broker)
}

func (b *statusableForBroker) GetSubscriptionName() string {
	return brokerresources.GenerateDecouplingSubscriptionName(b.broker)
}

var _ Statusable = (*statusableForChannel)(nil)

type statusableForChannel struct {
	ch *v1beta1.Channel
}

func StatusableFromChannel(c *v1beta1.Channel) Statusable {
	return &statusableForChannel{
		ch: c,
	}
}

func (c *statusableForChannel) Key() *config.CellTenantKey {
	return config.KeyFromChannel(c.ch)
}

func (c *statusableForChannel) MarkBrokerCellReady() {
	c.ch.Status.MarkBrokerCellReady()
}

func (c *statusableForChannel) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	c.ch.Status.MarkBrokerCellUnknown(reason, format, args...)
}

func (c *statusableForChannel) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	c.ch.Status.MarkBrokerCellFailed(reason, format, args...)
}

func (c *statusableForChannel) SetAddress(url *apis.URL) {
	c.ch.Status.SetAddress(url)
}

func (c *statusableForChannel) Object() runtime.Object {
	return c.ch
}

func (c *statusableForChannel) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &c.ch.Status
}

func (c *statusableForChannel) GetLabels() map[string]string {
	return map[string]string{
		"resource":  "channels",
		"namespace": c.ch.Namespace,
		"name":      c.ch.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (c *statusableForChannel) GetTopicID() string {
	return channelresources.GenerateDecouplingTopicName(c.ch)
}

func (c *statusableForChannel) GetSubscriptionName() string {
	return channelresources.GenerateDecouplingSubscriptionName(c.ch)
}
