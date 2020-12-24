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
package gcpcelladdressable

import (
	"context"
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/knative-gcp/pkg/broker/config"

	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	brokercellresources "github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	inteventslisters "github.com/google/knative-gcp/pkg/client/listers/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	reconcilerutilspubsub "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub"
	"github.com/google/knative-gcp/pkg/utils"
)

const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerCellCreated = "BrokerCellCreated"
)

type GCPCellAddressableReconciler struct {
	*reconciler.Base

	// listers index properties about resources
	BrokerCellLister inteventslisters.BrokerCellLister

	ProjectID string

	// pubsubClient is used as the Pubsub client when present.
	PubsubClient *pubsub.Client

	DataresidencyStore *dataresidency.Store
}

func (r *GCPCellAddressableReconciler) ReconcileGCPCellAddressable(ctx context.Context, b BrokerCellStatusable) error {
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

func (r *GCPCellAddressableReconciler) FinalizeGCPCellAddressable(ctx context.Context, b BrokerCellStatusable) error {
	if err := r.deleteDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("failed to delete Pub/Sub topic: %v", err)
	}
	return nil
}

func (r *GCPCellAddressableReconciler) reconcileDecouplingTopicAndSubscription(ctx context.Context, b BrokerCellStatusable) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling decoupling topic", zap.Any("broker", b))
	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		b.StatusUpdater().MarkTopicUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		b.StatusUpdater().MarkSubscriptionUnknown("ProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}
	// Set the projectID in the status.
	//TODO uncomment when eventing webhook allows this
	//b.Status.ProjectID = projectID

	client, err := r.getClientOrCreateNew(ctx, projectID, b.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	// Check if topic exists, and if not, create it.
	topicID := b.GetTopicID()
	topicConfig := &pubsub.TopicConfig{Labels: b.GetLabels()}
	if r.DataresidencyStore != nil {
		if dataresidencyConfig := r.DataresidencyStore.Load(); dataresidencyConfig != nil {
			if dataresidencyConfig.DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig) {
				logging.FromContext(ctx).Debug("Updated Topic Config AllowedPersistenceRegions for Broker", zap.Any("topicConfig", *topicConfig))
			}
		}
	}
	topic, err := pubsubReconciler.ReconcileTopic(ctx, topicID, topicConfig, b.Object(), b.StatusUpdater())
	if err != nil {
		return err
	}
	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.TopicID = topic.ID()

	// Check if PullSub exists, and if not, create it.
	subID := b.GetSubscriptionName()
	subConfig := pubsub.SubscriptionConfig{
		Topic:  topic,
		Labels: b.GetLabels(),
		//TODO(grantr): configure these settings?
		// AckDeadline
		// RetentionDuration
	}
	if _, err := pubsubReconciler.ReconcileSubscription(ctx, subID, subConfig, b.Object(), b.StatusUpdater()); err != nil {
		return err
	}

	// TODO(grantr): this isn't actually persisted due to webhook issues.
	//TODO uncomment when eventing webhook allows this
	//b.Status.SubscriptionID = sub.ID()

	return nil
}

func (r *GCPCellAddressableReconciler) deleteDecouplingTopicAndSubscription(ctx context.Context, b BrokerCellStatusable) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling topic")

	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		b.StatusUpdater().MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %v", err)
		b.StatusUpdater().MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, b.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := b.GetTopicID()
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, b.Object(), b.StatusUpdater()))
	subID := b.GetSubscriptionName()
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, b.Object(), b.StatusUpdater()))

	return err
}

// createPubsubClientFn is a function for pubsub client creation. Changed in testing only.
var createPubsubClientFn reconcilerutilspubsub.CreateFn = pubsub.NewClient

// getClientOrCreateNew Return the pubsubCient if it is valid, otherwise it tries to create a new client
// and register it for later usage.
func (r *GCPCellAddressableReconciler) getClientOrCreateNew(ctx context.Context, projectID string, b reconcilerutilspubsub.StatusUpdater) (*pubsub.Client, error) {
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

// ensureBrokerCellExists creates a BrokerCell if it doesn't exist, and update broker status based on brokercell status.
func (r *GCPCellAddressableReconciler) ensureBrokerCellExists(ctx context.Context, b BrokerCellStatusable) error {
	var bc *inteventsv1alpha1.BrokerCell
	var err error
	// TODO(#866) Get brokercell based on the label (or annotation) on the broker.
	bcNS := system.Namespace()
	bcName := resources.DefaultBrokerCellName
	bc, err = r.BrokerCellLister.BrokerCells(bcNS).Get(bcName)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Error("Error getting BrokerCell", zap.String("namespace", bcNS), zap.String("brokerCell", bcName), zap.Error(err))
		b.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get BrokerCell %s/%s", bcNS, bcName)
		return err
	}

	if apierrs.IsNotFound(err) {
		want := resources.CreateBrokerCell(nil)
		bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Create(ctx, want, metav1.CreateOptions{})
		if err != nil && !apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Error("Error creating brokerCell", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name), zap.Error(err))
			b.MarkBrokerCellFailed("BrokerCellCreationFailed", "Failed to create BrokerCell %s/%s", want.Namespace, want.Name)
			return err
		}
		if apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Info("BrokerCell already exists", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name))
			// There can be a race condition where the informer is not updated. In this case we directly
			// read from the API server.
			bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to get the BrokerCell from the API server", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name), zap.Error(err))
				b.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get BrokerCell %s/%s", want.Namespace, want.Name)
				return err
			}
		}
		if err == nil {
			r.Recorder.Eventf(b.Object(), corev1.EventTypeNormal, brokerCellCreated, "Created BrokerCell %s/%s", bc.Namespace, bc.Name)
		}
	}

	if bc.Status.IsReady() {
		b.MarkBrokerCellReady()
	} else {
		b.MarkBrokerCellUnknown("BrokerCellNotReady", "BrokerCell %s/%s is not ready", bc.Namespace, bc.Name)
	}

	//TODO(#1019) Use the IngressTemplate of brokercell.
	ingressServiceName := brokercellresources.Name(bc.Name, brokercellresources.IngressName)
	b.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, bc.Namespace),
		Path:   b.Key().PersistenceString(),
	})

	return nil
}

type BrokerCellStatusable interface {
	Key() config.GCPCellAddressableKey
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

var _ BrokerCellStatusable = (*brokerCellStatusableBroker)(nil)

type brokerCellStatusableBroker struct {
	broker *brokerv1beta1.Broker
}

func BrokerCellStatusableFromBroker(b *brokerv1beta1.Broker) BrokerCellStatusable {
	return &brokerCellStatusableBroker{
		broker: b,
	}
}

func (b brokerCellStatusableBroker) Key() config.GCPCellAddressableKey {
	return config.KeyFromBroker(b.broker)
}

func (b brokerCellStatusableBroker) MarkBrokerCellReady() {
	b.broker.Status.MarkBrokerCellReady()
}

func (b brokerCellStatusableBroker) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellUnknown(reason, format, args...)
}

func (b brokerCellStatusableBroker) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	b.broker.Status.MarkBrokerCellFailed(reason, format, args...)
}

func (b brokerCellStatusableBroker) SetAddress(url *apis.URL) {
	b.broker.Status.SetAddress(url)
}

func (b brokerCellStatusableBroker) Object() runtime.Object {
	return b.broker
}

func (b brokerCellStatusableBroker) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &b.broker.Status
}

func (b brokerCellStatusableBroker) GetLabels() map[string]string {
	return map[string]string{
		"resource":     "brokers",
		"broker_class": brokerv1beta1.BrokerClass,
		"namespace":    b.broker.Namespace,
		"name":         b.broker.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (b brokerCellStatusableBroker) GetTopicID() string {
	return resources.GenerateDecouplingTopicName(b.broker)
}

func (b brokerCellStatusableBroker) GetSubscriptionName() string {
	return resources.GenerateDecouplingSubscriptionName(b.broker)
}

var _ BrokerCellStatusable = (*brokerCellStatusableChannel)(nil)

type brokerCellStatusableChannel struct {
	channel *v1beta1.Channel
}

func (c brokerCellStatusableChannel) Key() config.GCPCellAddressableKey {
	return config.KeyFromChannel(c.channel)
}

func (c brokerCellStatusableChannel) MarkBrokerCellReady() {
	// TODO Make the channel have BrokerCell statuses instead of these fake topic ones.
	c.channel.Status.MarkTopicReady()
}

func (c brokerCellStatusableChannel) MarkBrokerCellUnknown(reason, format string, args ...interface{}) {
	c.channel.Status.MarkTopicUnknown(reason, format, args...)
}

func (c brokerCellStatusableChannel) MarkBrokerCellFailed(reason, format string, args ...interface{}) {
	c.channel.Status.MarkTopicFailed(reason, format, args...)
}

func (c brokerCellStatusableChannel) SetAddress(url *apis.URL) {
	c.channel.Status.SetAddress(url)
}

func (c brokerCellStatusableChannel) Object() runtime.Object {
	return c.channel
}
func (c brokerCellStatusableChannel) StatusUpdater() reconcilerutilspubsub.StatusUpdater {
	return &c.channel.Status
}

func (c brokerCellStatusableChannel) GetLabels() map[string]string {
	return map[string]string{
		"resource":  "channels",
		"namespace": c.channel.Namespace,
		"name":      c.channel.Name,
		//TODO add resource labels, but need to be sanitized: https://cloud.google.com/pubsub/docs/labels#requirements
	}
}

func (c brokerCellStatusableChannel) GetTopicID() string {
	return resources.GenerateChannelDecouplingTopicName(c.channel)
}

func (c brokerCellStatusableChannel) GetSubscriptionName() string {
	return resources.GenerateChannelDecouplingSubscriptionName(c.channel)
}

func BrokerCellStatusableFromChannel(c *v1beta1.Channel) BrokerCellStatusable {
	return &brokerCellStatusableChannel{
		channel: c,
	}
}
