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

package celltenant

import (
	"fmt"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
)

// CellTenantDeletingTarget is the interface that the TargetReconciler takes for FinalizeKind().
type CellTenantDeletingTarget interface {
	Object() runtime.Object
	StatusUpdater() reconcilerutilspubsub.StatusUpdater
	GetTopicID() string
	GetSubscriptionName() string
}

// CellTenantTarget is the interface that the TargetReconciler takes for ReconcileKind().
type CellTenantTarget interface {
	CellTenantDeletingTarget
	GetLabels() map[string]string
	DeliverySpec() *eventingduckv1beta1.DeliverySpec
	SetStatusProjectID(projectID string)
}

var _ CellTenantTarget = (*cellTenantTargetForTrigger)(nil)

type cellTenantTargetForTrigger struct {
	trigger      *brokerv1beta1.Trigger
	deliverySpec *eventingduckv1beta1.DeliverySpec
}

// CellTenantTargetFromTrigger creates a CellTenantTarget for the given Trigger and associated
// Broker's deliverySpec.
func CellTenantTargetFromTrigger(t *brokerv1beta1.Trigger, deliverySpec *eventingduckv1beta1.DeliverySpec) CellTenantTarget {
	return &cellTenantTargetForTrigger{
		trigger:      t,
		deliverySpec: deliverySpec,
	}
}

func (t *cellTenantTargetForTrigger) Key222() *config.TargetKey {
	return config.KeyFromTrigger(t.trigger)
}

func (t *cellTenantTargetForTrigger) Object() runtime.Object {
	return t.trigger
}

func (t *cellTenantTargetForTrigger) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &t.trigger.Status
}

func (t *cellTenantTargetForTrigger) GetLabels() map[string]string {
	return map[string]string{
		"resource":  "triggers",
		"namespace": t.trigger.Namespace,
		"name":      t.trigger.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (t *cellTenantTargetForTrigger) GetTopicID() string {
	return resources.GenerateRetryTopicName(t.trigger)
}

func (t *cellTenantTargetForTrigger) GetSubscriptionName() string {
	return resources.GenerateRetrySubscriptionName(t.trigger)
}

func (t *cellTenantTargetForTrigger) DeliverySpec() *eventingduckv1beta1.DeliverySpec {
	return t.deliverySpec
}

func (t *cellTenantTargetForTrigger) SetStatusProjectID(projectID string) {
	// TODO uncomment when eventing webhook allows this
	// t.trigger.Status.ProjectID = projectID
}

var _ reconcilerutilspubsub.StatusUpdater = (*SubscriberStatus)(nil)

type SubscriberStatus struct {
	topicStatus         corev1.ConditionStatus
	topicMessage        string
	subscriptionStatus  corev1.ConditionStatus
	subscriptionMessage string
}

func (s SubscriberStatus) MarkTopicFailed(_, format string, args ...interface{}) {
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

// CellTenantStatusable is the interface used by the CellTenantReconciler.
type CellTenantStatusable interface {
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

var _ CellTenantStatusable = (*cellTenantStatusableForBroker)(nil)

type cellTenantStatusableForBroker struct {
	broker *brokerv1beta1.Broker
}

func CellTenantStatusableFromBroker(b *brokerv1beta1.Broker) CellTenantStatusable {
	return &cellTenantStatusableForBroker{
		broker: b,
	}
}

func (b cellTenantStatusableForBroker) Key() *config.CellTenantKey {
	return config.KeyFromBroker(b.broker)
}

func (b cellTenantStatusableForBroker) MarkBrokerCellReady() {
	b.broker.Status.MarkBrokerCellReady()
}

func (b cellTenantStatusableForBroker) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellUnknown(reason, format, args...)
}

func (b cellTenantStatusableForBroker) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellFailed(reason, format, args...)
}

func (b cellTenantStatusableForBroker) SetAddress(url *apis.URL) {
	b.broker.Status.SetAddress(url)
}

func (b cellTenantStatusableForBroker) Object() runtime.Object {
	return b.broker
}

func (b cellTenantStatusableForBroker) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &b.broker.Status
}

func (b cellTenantStatusableForBroker) GetLabels() map[string]string {
	return map[string]string{
		"resource":     "brokers",
		"broker_class": brokerv1beta1.BrokerClass,
		"namespace":    b.broker.Namespace,
		"name":         b.broker.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (b cellTenantStatusableForBroker) GetTopicID() string {
	return resources.GenerateDecouplingTopicName(b.broker)
}

func (b cellTenantStatusableForBroker) GetSubscriptionName() string {
	return resources.GenerateDecouplingSubscriptionName(b.broker)
}
