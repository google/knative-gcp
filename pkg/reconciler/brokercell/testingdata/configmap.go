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

package testingdata

import (
	"testing"

	channelresources "github.com/google/knative-gcp/pkg/reconciler/messaging/channel/resources"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	brokerresources "github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	corev1 "k8s.io/api/core/v1"
)

func EmptyConfig(t *testing.T, bc *intv1alpha1.BrokerCell) *corev1.ConfigMap {
	cm, _ := resources.MakeTargetsConfig(bc, memory.NewEmptyTargets())
	return cm
}

type BrokerCellObjects struct {
	BrokersToTriggers map[*brokerv1beta1.Broker][]*brokerv1beta1.Trigger
	Channels          []*v1beta1.Channel
}

func Config(bc *intv1alpha1.BrokerCell, bco BrokerCellObjects) *corev1.ConfigMap {
	targets := &config.TargetsConfig{
		CellTenants: map[string]*config.CellTenant{},
	}

	for broker, triggers := range bco.BrokersToTriggers {
		addBroker(targets, broker, triggers)
	}
	for _, channel := range bco.Channels {
		addChannel(targets, channel)
	}

	memoryTargets := memory.NewTargets(targets)
	cm, _ := resources.MakeTargetsConfig(bc, memoryTargets)
	return cm
}

func addBroker(targets *config.TargetsConfig, broker *brokerv1beta1.Broker, triggers []*brokerv1beta1.Trigger) {
	if broker == nil {
		return
	}
	state := config.State_UNKNOWN
	if broker.Status.IsReady() {
		state = config.State_READY
	}
	brokerQueueState := config.State_UNKNOWN
	if broker.Status.GetCondition(brokerv1beta1.BrokerConditionTopic).IsTrue() && broker.Status.GetCondition(brokerv1beta1.BrokerConditionSubscription).IsTrue() {
		brokerQueueState = config.State_READY
	}
	brokerConfig := &config.CellTenant{
		Id:        string(broker.UID),
		Type:      config.CellTenantType_BROKER,
		Name:      broker.Name,
		Namespace: broker.Namespace,
		Address:   broker.Status.Address.URL.String(),
		DecoupleQueue: &config.Queue{
			Topic:        brokerresources.GenerateDecouplingTopicName(broker),
			Subscription: brokerresources.GenerateDecouplingSubscriptionName(broker),
			State:        brokerQueueState,
		},
		Targets: make(map[string]*config.Target),
		State:   state,
	}
	for _, trigger := range triggers {
		var filterAttributes map[string]string
		if trigger.Spec.Filter != nil && trigger.Spec.Filter.Attributes != nil {
			filterAttributes = trigger.Spec.Filter.Attributes
		}
		brokerConfig.Targets[trigger.Name] = &config.Target{
			Id:             string(trigger.UID),
			Name:           trigger.Name,
			Namespace:      trigger.Namespace,
			CellTenantType: config.CellTenantType_BROKER,
			CellTenantName: trigger.Spec.Broker,
			ReplyAddress:   broker.Status.Address.URL.String(),
			Address:        trigger.Status.SubscriberURI.String(),
			RetryQueue: &config.Queue{
				Topic:        brokerresources.GenerateRetryTopicName(trigger),
				Subscription: brokerresources.GenerateRetrySubscriptionName(trigger),
			},
			State:            state,
			FilterAttributes: filterAttributes,
		}
	}
	targets.CellTenants[brokerConfig.Key().PersistenceString()] = brokerConfig
}

func addChannel(targets *config.TargetsConfig, channel *v1beta1.Channel) {
	if channel == nil {
		return
	}
	state := config.State_UNKNOWN
	if channel.Status.IsReady() {
		state = config.State_READY
	}
	queueState := config.State_UNKNOWN
	if channel.Status.GetCondition(brokerv1beta1.BrokerConditionTopic).IsTrue() && channel.Status.GetCondition(brokerv1beta1.BrokerConditionSubscription).IsTrue() {
		queueState = config.State_READY
	}
	cellTenant := &config.CellTenant{
		Id:        string(channel.UID),
		Type:      config.CellTenantType_CHANNEL,
		Name:      channel.Name,
		Namespace: channel.Namespace,
		Address:   channel.Status.Address.URL.String(),
		DecoupleQueue: &config.Queue{
			Topic:        channelresources.GenerateDecouplingTopicName(channel),
			Subscription: channelresources.GenerateDecouplingSubscriptionName(channel),
			State:        queueState,
		},
		Targets: make(map[string]*config.Target),
		State:   state,
	}
	if channel.Spec.SubscribableSpec != nil {
		for _, s := range channel.Spec.Subscribers {
			cellTenant.Targets[string(s.UID)] = &config.Target{
				Id:             string(s.UID),
				Name:           string(s.UID),
				Namespace:      channel.Namespace,
				CellTenantName: channel.Name,
				CellTenantType: config.CellTenantType_CHANNEL,
				Address:        s.SubscriberURI.String(),
				RetryQueue: &config.Queue{
					Topic:        channelresources.GenerateSubscriberRetryTopicName(channel, s.UID),
					Subscription: channelresources.GenerateSubscriberRetrySubscriptionName(channel, s.UID),
				},
				State:        config.State_READY,
				ReplyAddress: s.ReplyURI.String(),
			}
		}
	}
	targets.CellTenants[cellTenant.Key().PersistenceString()] = cellTenant
}
