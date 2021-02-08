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

// Package broker implements the Broker controller and reconciler reconciling
// Brokers and Triggers.
package celltenant

import (
	"context"
	"fmt"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"

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

// Reconciler implements controller.Reconciler for CellTenants.
type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	BrokerCellLister inteventslisters.BrokerCellLister

	ProjectID string

	// pubsubClient is used as the Pubsub client when present.
	PubsubClient *pubsub.Client

	DataresidencyStore *dataresidency.Store

	// clusterRegion is the region where GKE is running.
	ClusterRegion string
}

func (r *Reconciler) ReconcileGCPCellTenant(ctx context.Context, b Statusable) error {
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

func (r *Reconciler) FinalizeGCPCellTenant(ctx context.Context, b Statusable) error {
	if err := r.deleteDecouplingTopicAndSubscription(ctx, b); err != nil {
		return fmt.Errorf("failed to delete Pub/Sub topic: %v", err)
	}
	return nil
}

func (r *Reconciler) reconcileDecouplingTopicAndSubscription(ctx context.Context, b Statusable) error {
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

	r.ClusterRegion, err = utils.ClusterRegion(r.ClusterRegion, metadataClient.NewDefaultMetadataClient)
	if err != nil {
		logger.Error("Failed to get cluster region: ", zap.Error(err))
		return err
	}

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
		if r.DataresidencyStore.Load().DataResidencyDefaults.ComputeAllowedPersistenceRegions(topicConfig, r.ClusterRegion) {
			logger.Debug("Updated Topic Config AllowedPersistenceRegions for Broker", zap.Any("topicConfig", *topicConfig))
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

func (r *Reconciler) deleteDecouplingTopicAndSubscription(ctx context.Context, s Statusable) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Deleting decoupling topic")

	// get ProjectID from metadata if projectID isn't set
	projectID, err := utils.ProjectIDOrDefault(r.ProjectID)
	if err != nil {
		logger.Error("Failed to find project id", zap.Error(err))
		s.StatusUpdater().MarkTopicUnknown("FinalizeTopicProjectIdNotFound", "Failed to find project id: %v", err)
		s.StatusUpdater().MarkSubscriptionUnknown("FinalizeSubscriptionProjectIdNotFound", "Failed to find project id: %v", err)
		return err
	}

	client, err := r.getClientOrCreateNew(ctx, projectID, s.StatusUpdater())
	if err != nil {
		logger.Error("Failed to create Pub/Sub client", zap.Error(err))
		return err
	}
	pubsubReconciler := reconcilerutilspubsub.NewReconciler(client, r.Recorder)

	// Delete topic if it exists. Pull subscriptions continue pulling from the
	// topic until deleted themselves.
	topicID := s.GetTopicID()
	err = multierr.Append(nil, pubsubReconciler.DeleteTopic(ctx, topicID, s.Object(), s.StatusUpdater()))
	subID := s.GetSubscriptionName()
	err = multierr.Append(err, pubsubReconciler.DeleteSubscription(ctx, subID, s.Object(), s.StatusUpdater()))

	return err
}

// CreatePubsubClientFn is a function for pubsub client creation. Changed in testing only.
// TODO Stop exporting this once unit tests are migrated from the Broker and Trigger reconcilers to
// the CellTenant and Target reconcilers.
var CreatePubsubClientFn reconcilerutilspubsub.CreateFn = pubsub.NewClient

// getClientOrCreateNew Return the pubsubCient if it is valid, otherwise it tries to create a new client
// and register it for later usage.
func (r *Reconciler) getClientOrCreateNew(ctx context.Context, projectID string, su reconcilerutilspubsub.StatusUpdater) (*pubsub.Client, error) {
	if r.PubsubClient != nil {
		return r.PubsubClient, nil
	}
	client, err := CreatePubsubClientFn(ctx, projectID)
	if err != nil {
		su.MarkTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		su.MarkSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: %v", err)
		return nil, err
	}
	// Register the client for next run
	r.PubsubClient = client
	return client, nil
}

// ensureBrokerCellExists creates a BrokerCell if it doesn't exist, and update broker status based on brokercell status.
func (r *Reconciler) ensureBrokerCellExists(ctx context.Context, s Statusable) error {
	var bc *inteventsv1alpha1.BrokerCell
	var err error
	// TODO(#866) Get brokercell based on the label (or annotation) on the broker.
	bcNS := system.Namespace()
	bcName := resources.DefaultBrokerCellName
	bc, err = r.BrokerCellLister.BrokerCells(bcNS).Get(bcName)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Error("Error getting BrokerCell", zap.String("namespace", bcNS), zap.String("brokerCell", bcName), zap.Error(err))
		s.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get BrokerCell %s/%s", bcNS, bcName)
		return err
	}

	if apierrs.IsNotFound(err) {
		want := resources.CreateBrokerCell(nil)
		bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Create(ctx, want, metav1.CreateOptions{})
		if err != nil && !apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Error("Error creating brokerCell", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name), zap.Error(err))
			s.MarkBrokerCellFailed("BrokerCellCreationFailed", "Failed to create BrokerCell %s/%s", want.Namespace, want.Name)
			return err
		}
		if apierrs.IsAlreadyExists(err) {
			logging.FromContext(ctx).Info("BrokerCell already exists", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name))
			// There can be a race condition where the informer is not updated. In this case we directly
			// read from the API server.
			bc, err = r.RunClientSet.InternalV1alpha1().BrokerCells(want.Namespace).Get(ctx, want.Name, metav1.GetOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to get the BrokerCell from the API server", zap.String("namespace", want.Namespace), zap.String("brokerCell", want.Name), zap.Error(err))
				s.MarkBrokerCellUnknown("BrokerCellUnknown", "Failed to get BrokerCell %s/%s", want.Namespace, want.Name)
				return err
			}
		}
		if err == nil {
			r.Recorder.Eventf(s.Object(), corev1.EventTypeNormal, brokerCellCreated, "Created BrokerCell %s/%s", bc.Namespace, bc.Name)
		}
	}

	if bc.Status.IsReady() {
		s.MarkBrokerCellReady()
	} else {
		s.MarkBrokerCellUnknown("BrokerCellNotReady", "BrokerCell %s/%s is not ready", bc.Namespace, bc.Name)
	}

	//TODO(#1019) Use the IngressTemplate of brokercell.
	ingressServiceName := brokercellresources.Name(bc.Name, brokercellresources.IngressName)
	s.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   network.GetServiceHostname(ingressServiceName, bc.Namespace),
		Path:   "/" + s.Key().PersistenceString(),
	})

	return nil
}
