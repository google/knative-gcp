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

func Config(t *testing.T, bc *intv1alpha1.BrokerCell, broker *brokerv1beta1.Broker, triggers ...*brokerv1beta1.Trigger) *corev1.ConfigMap {
	// construct triggers config
	targets := make(map[string]*config.Target, len(triggers))
	for _, t := range triggers {
		state := config.State_UNKNOWN
		if t.Status.IsReady() {
			state = config.State_READY
		}
		var filterAttributes map[string]string
		if t.Spec.Filter != nil && t.Spec.Filter.Attributes != nil {
			filterAttributes = t.Spec.Filter.Attributes
		}
		target := &config.Target{
			Id:        string(t.UID),
			Name:      t.Name,
			Namespace: t.Namespace,
			Broker:    broker.Name,
			Address:   t.Status.SubscriberURI.String(),
			RetryQueue: &config.Queue{
				Topic:        brokerresources.GenerateRetryTopicName(t),
				Subscription: brokerresources.GenerateRetrySubscriptionName(t),
			},
			State:            state,
			FilterAttributes: filterAttributes,
		}

		targets[t.Name] = target
	}

	// construct broker config
	state := config.State_UNKNOWN
	if broker.Status.IsReady() {
		state = config.State_READY
	}
	brokerQueueState := config.State_UNKNOWN
	if broker.Status.GetCondition(brokerv1beta1.BrokerConditionTopic).IsTrue() && broker.Status.GetCondition(brokerv1beta1.BrokerConditionSubscription).IsTrue() {
		brokerQueueState = config.State_READY
	}
	brokerConfig := &config.Broker{
		Id:        string(broker.UID),
		Name:      broker.Name,
		Namespace: broker.Namespace,
		Address:   broker.Status.Address.URL.String(),
		DecoupleQueue: &config.Queue{
			Topic:        brokerresources.GenerateDecouplingTopicName(broker),
			Subscription: brokerresources.GenerateDecouplingSubscriptionName(broker),
			State:        brokerQueueState,
		},
		Targets: targets,
		State:   state,
	}
	bt := &config.TargetsConfig{
		Brokers: map[string]*config.Broker{
			brokerConfig.Key(): brokerConfig,
		},
	}
	brokerTargets := memory.NewTargets(bt)
	cm, _ := resources.MakeTargetsConfig(bc, brokerTargets)
	return cm
}
