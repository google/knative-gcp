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

package brokercell

import (
	"context"
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"

	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	brokerresources "github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	channelresources "github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"
	"github.com/google/knative-gcp/pkg/reconciler/utils"
	"github.com/google/knative-gcp/pkg/reconciler/utils/volume"
)

const (
	configFailed = "BrokerTargetsConfigFailed"
)

func (r *Reconciler) reconcileConfig(ctx context.Context, bc *intv1alpha1.BrokerCell) error {
	// Start with a fresh config and add into it. This approach is straightforward and reliable,
	// however not efficient if there are too many triggers/subscriptions. If performance becomes
	// an issue, we can consider maintaining 2 queues for updated brokers and triggers, and only
	// update the config for updated triggers/subscriptions.
	targets := memory.NewEmptyTargets()

	err := r.addBrokersAndTriggersToTargets(ctx, bc, targets)
	if err != nil {
		return fmt.Errorf("unable to add Broker and Triggers to targets: %w", err)
	}

	err = r.addChannelsToTargets(ctx, bc, targets)
	if err != nil {
		return fmt.Errorf("unable to add Channels to targets: %w", err)
	}

	if err := r.updateTargetsConfig(ctx, bc, targets); err != nil {
		logging.FromContext(ctx).Error("Failed to update broker targets configmap", zap.Error(err))
		bc.Status.MarkTargetsConfigFailed(configFailed, "failed to update configmap: %v", err)
		return err
	}
	bc.Status.MarkTargetsConfigReady()
	return nil
}

// addBrokersAndTriggersToTargets adds all Brokers that are associated with the `bc` BrokerCell to
// `targets`, along with all Triggers that target those Brokers.
func (r *Reconciler) addBrokersAndTriggersToTargets(ctx context.Context, bc *intv1alpha1.BrokerCell, targets config.Targets) error {
	// TODO(#866) Only select brokers that point to this brokercell by label selector once the
	// webhook assigns the brokercell label, i.e.,
	// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
	brokers, err := r.brokerLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Failed to list brokers", zap.Error(err))
		bc.Status.MarkTargetsConfigFailed(configFailed, "failed to list brokers: %v", err)
		return err
	}
	for _, broker := range brokers {
		if !utils.BrokerClassFilter(broker) {
			continue
		}
		// Filter by `eventing.knative.dev/broker: <name>` here
		// to get only the triggers for this broker. The trigger webhook will
		// ensure that triggers are always labeled with their broker name.
		triggers, err := r.triggerLister.Triggers(broker.Namespace).List(labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: broker.Name}))
		if err != nil {
			logging.FromContext(ctx).Error("Failed to list triggers", zap.String("Broker", broker.Name), zap.Error(err))
			bc.Status.MarkTargetsConfigFailed(configFailed, "failed to list triggers for broker %v: %v", broker.Name, err)
			return err
		}
		addBrokerAndTriggersToConfig(ctx, broker, triggers, targets)
	}
	return nil
}

// addBrokerAndTriggersToConfig reconstructs the data entry for the given broker and adds it to targets-config.
func addBrokerAndTriggersToConfig(_ context.Context, b *brokerv1beta1.Broker, triggers []*brokerv1beta1.Trigger, brokerTargets config.Targets) {
	// TODO Maybe get rid of GCPCellAddressableMutation and add Delete() and Upsert(broker) methods to TargetsConfig. Now we always
	//  delete or update the entire broker entry and we don't need partial updates per trigger.
	// The code can be simplified to r.targetsConfig.Upsert(brokerConfigEntry)
	brokerTargets.MutateCellTenant(config.KeyFromBroker(b), func(m config.CellTenantMutation) {
		// First delete the broker entry.
		m.Delete()

		brokerQueueState := config.State_UNKNOWN
		// Set broker decouple queue to be ready only when both the topic and pull subscription are ready.
		// PubSub will drop messages published to a topic if there is no subscription.
		if b.Status.GetCondition(brokerv1beta1.BrokerConditionTopic).IsTrue() && b.Status.GetCondition(brokerv1beta1.BrokerConditionSubscription).IsTrue() {
			brokerQueueState = config.State_READY
		}
		// Then reconstruct the broker entry and insert it
		m.SetID(string(b.UID))
		m.SetAddress(b.Status.Address.URL.String())
		m.SetDecoupleQueue(&config.Queue{
			Topic:        brokerresources.GenerateDecouplingTopicName(b),
			Subscription: brokerresources.GenerateDecouplingSubscriptionName(b),
			State:        brokerQueueState,
		})
		if b.Status.IsReady() {
			m.SetState(config.State_READY)
		} else {
			m.SetState(config.State_UNKNOWN)
		}

		// Insert each Trigger to the config.
		for _, t := range triggers {
			if t.Spec.Broker == b.Name {
				target := &config.Target{
					Id:             string(t.UID),
					Name:           t.Name,
					Namespace:      t.Namespace,
					CellTenantType: config.CellTenantType_BROKER,
					CellTenantName: b.Name,
					ReplyAddress:   b.Status.Address.URL.String(),
					Address:        t.Status.SubscriberURI.String(),
					RetryQueue: &config.Queue{
						Topic:        brokerresources.GenerateRetryTopicName(t),
						Subscription: brokerresources.GenerateRetrySubscriptionName(t),
					},
				}
				if t.Spec.Filter != nil && t.Spec.Filter.Attributes != nil {
					target.FilterAttributes = t.Spec.Filter.Attributes
				}
				// TODO(#939) May need to use "data plane readiness" for trigger in stead of the
				//  overall status, see https://github.com/google/knative-gcp/issues/939#issuecomment-644337937
				if t.Status.IsReady() {
					target.State = config.State_READY
				} else {
					target.State = config.State_UNKNOWN
				}
				m.UpsertTargets(target)
			}
		}
	})
}
func (r *Reconciler) addChannelsToTargets(ctx context.Context, bc *intv1alpha1.BrokerCell, targets config.Targets) error {
	// TODO(#866) Only select Channels that point to this brokercell by label selector once the
	// webhook assigns the brokercell label, i.e.,
	// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
	channels, err := r.channelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Failed to list Channels", zap.Error(err))
		bc.Status.MarkTargetsConfigFailed(configFailed, "failed to list channels: %v", err)
		return err
	}
	for _, channel := range channels {
		addChannelToConfig(ctx, channel, targets)
	}
	return nil
}

// addChannelToConfig reconstructs the data entry for the given Channel and adds it to targets-config.
func addChannelToConfig(_ context.Context, c *v1beta1.Channel, targets config.Targets) {
	if c.Status.Address == nil {
		// The address hasn't been set. The Channel reconciler will get to it. At which point the
		// Channel will be modified, so the BrokerCell will reconcile again. For now, ignore this
		// Channel.
		return
	}
	// TODO Maybe get rid of CellTenantMutation and add Delete() and Upsert(broker) methods to TargetsConfig. Now we
	// always delete or update the entire tenant entry and we don't need partial updates per trigger.
	// The code can be simplified to r.targetsConfig.Upsert(brokerConfigEntry)
	targets.MutateCellTenant(config.KeyFromChannel(c), func(m config.CellTenantMutation) {
		// First delete the Channel entry.
		m.Delete()

		queueState := config.State_UNKNOWN
		// Set decouple queue to be ready only when both the topic and pull subscription are ready.
		// PubSub will drop messages published to a topic if there is no subscription.
		if cs := c.Status; cs.GetCondition(v1beta1.ChannelConditionTopicReady).IsTrue() &&
			cs.GetCondition(v1beta1.ChannelConditionSubscription).IsTrue() {
			queueState = config.State_READY
		}

		// Then reconstruct the tenant entry and insert it.
		m.SetID(string(c.UID))
		m.SetAddress(c.Status.Address.URL.String())
		m.SetDecoupleQueue(&config.Queue{
			Topic:        channelresources.GenerateDecouplingTopicName(c),
			Subscription: channelresources.GenerateDecouplingSubscriptionName(c),
			State:        queueState,
		})
		if c.Status.IsReady() {
			m.SetState(config.State_READY)
		} else {
			m.SetState(config.State_UNKNOWN)
		}

		if c.Spec.SubscribableSpec == nil {
			return
		}
		for _, s := range c.Spec.SubscribableSpec.Subscribers {
			target := &config.Target{
				Id:        string(s.UID),
				Namespace: c.Namespace,
				// Name is used as a key to look up this target in the Fanout and Retry Pods. Be
				// very careful changing it, as it might lead to event loss during upgrade (while
				// the code is new, but the config is old).
				Name:           string(s.UID),
				CellTenantType: config.CellTenantType_CHANNEL,
				CellTenantName: c.Name,
				Address:        s.SubscriberURI.String(),
				ReplyAddress:   s.ReplyURI.String(),
				RetryQueue: &config.Queue{
					Topic:        channelresources.GenerateSubscriberRetryTopicName(c, s.UID),
					Subscription: channelresources.GenerateSubscriberRetrySubscriptionName(c, s.UID),
				},
				// TODO(#939) May need to use "data plane readiness" for trigger in stead of the
				//  overall status, see https://github.com/google/knative-gcp/issues/939#issuecomment-644337937
				State: config.State_READY,
			}
			m.UpsertTargets(target)
		}
	})
}

//TODO all this stuff should be in a configmap variant of the config object
func (r *Reconciler) updateTargetsConfig(ctx context.Context, bc *intv1alpha1.BrokerCell, brokerTargets config.Targets) error {
	desired, err := resources.MakeTargetsConfig(bc, brokerTargets)
	if err != nil {
		return fmt.Errorf("error creating targets config: %w", err)
	}

	logging.FromContext(ctx).Debug("Current targets config", zap.Any("targetsConfig", brokerTargets.DebugString()))

	handlerFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { r.refreshPodVolume(ctx, bc) },
		UpdateFunc: func(oldObj, newObj interface{}) { r.refreshPodVolume(ctx, bc) },
		DeleteFunc: nil,
	}
	_, err = r.cmRec.ReconcileConfigMap(ctx, bc, desired, resources.TargetsConfigMapEqual, handlerFuncs)
	return err
}

func (r *Reconciler) refreshPodVolume(ctx context.Context, bc *intv1alpha1.BrokerCell) {
	if err := volume.UpdateVolumeGeneration(ctx, r.KubeClientSet, r.podLister, bc.Namespace, resources.CommonLabels(bc.Name)); err != nil {
		// Failing to update the annotation on the data plane pods means there
		// may be a longer propagation delay for the configmap volume to be
		// refreshed. But this is not treated as an error.
		logging.FromContext(ctx).Warn("Error updating annotation for data plane pods", zap.Error(err))
	}
}
