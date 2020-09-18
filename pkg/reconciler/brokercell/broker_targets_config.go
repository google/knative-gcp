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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/logging"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	brokerresources "github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	"github.com/google/knative-gcp/pkg/reconciler/utils/volume"
)

const (
	configFailed = "BrokerTargetsConfigFailed"
)

func (r *Reconciler) reconcileConfig(ctx context.Context, bc *intv1alpha1.BrokerCell) error {
	// TODO(#866) Only select brokers that point to this brokercell by label selector once the
	// webhook assigns the brokercell label, i.e.,
	// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
	brokers, err := r.brokerLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Failed to list brokers", zap.Error(err))
		bc.Status.MarkTargetsConfigFailed(configFailed, "failed to list brokers: %v", err)
		return err
	}
	// Start with a fresh config and add brokers/triggers into it. This approach is straightforward and reliable,
	// however not efficient if there are too many triggers. If performance becomes an issue, we can consider
	// maintaining 2 queues for updated brokers and triggers, and only update the config for updated brokers/triggers.
	brokerTargets := memory.NewEmptyTargets()
	for _, broker := range brokers {
		// Filter by `eventing.knative.dev/broker: <name>` here
		// to get only the triggers for this broker. The trigger webhook will
		// ensure that triggers are always labeled with their broker name.
		triggers, err := r.triggerLister.Triggers(broker.Namespace).List(labels.SelectorFromSet(map[string]string{eventing.BrokerLabelKey: broker.Name}))
		if err != nil {
			logging.FromContext(ctx).Error("Failed to list triggers", zap.String("Broker", broker.Name), zap.Error(err))
			bc.Status.MarkTargetsConfigFailed(configFailed, "failed to list triggers for broker %v: %v", broker.Name, err)
			return err
		}
		r.addToConfig(ctx, broker, triggers, brokerTargets)
	}
	if err := r.updateTargetsConfig(ctx, bc, brokerTargets); err != nil {
		logging.FromContext(ctx).Error("Failed to update broker targets configmap", zap.Error(err))
		bc.Status.MarkTargetsConfigFailed(configFailed, "failed to update configmap: %v", err)
		return err
	}
	bc.Status.MarkTargetsConfigReady()
	return nil
}

// addToConfig reconstructs the data entry for the given broker and add it to targets-config.
func (r *Reconciler) addToConfig(ctx context.Context, b *brokerv1beta1.Broker, triggers []*brokerv1beta1.Trigger, brokerTargets config.Targets) {
	// TODO Maybe get rid of BrokerMutation and add Delete() and Upsert(broker) methods to TargetsConfig. Now we always
	//  delete or update the entire broker entry and we don't need partial updates per trigger.
	// The code can be simplified to r.targetsConfig.Upsert(brokerConfigEntry)
	brokerTargets.MutateBroker(b.Namespace, b.Name, func(m config.BrokerMutation) {
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
					Id:        string(t.UID),
					Name:      t.Name,
					Namespace: t.Namespace,
					Broker:    b.Name,
					Address:   t.Status.SubscriberURI.String(),
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

//TODO all this stuff should be in a configmap variant of the config object
func (r *Reconciler) updateTargetsConfig(ctx context.Context, bc *intv1alpha1.BrokerCell, brokerTargets config.Targets) error {
	desired, err := resources.MakeTargetsConfig(bc, brokerTargets)
	if err != nil {
		return fmt.Errorf("error creating targets config: %w", err)
	}

	logging.FromContext(ctx).Debug("Current targets config", zap.Any("targetsConfig", brokerTargets.String()))

	handlerFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { r.refreshPodVolume(ctx, bc) },
		UpdateFunc: func(oldObj, newObj interface{}) { r.refreshPodVolume(ctx, bc) },
		DeleteFunc: nil,
	}
	_, err = r.cmRec.ReconcileConfigMap(bc, desired, resources.TargetsConfigMapEqual, handlerFuncs)
	return err
}

func (r *Reconciler) refreshPodVolume(ctx context.Context, bc *intv1alpha1.BrokerCell) {
	if err := volume.UpdateVolumeGeneration(r.KubeClientSet, r.podLister, bc.Namespace, resources.CommonLabels(bc.Name)); err != nil {
		// Failing to update the annotation on the data plane pods means there
		// may be a longer propagation delay for the configmap volume to be
		// refreshed. But this is not treated as an error.
		logging.FromContext(ctx).Warn("Error updating annotation for data plane pods", zap.Error(err))
	}
}
