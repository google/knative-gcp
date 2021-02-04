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

package controller

import (
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config"
	. "github.com/google/knative-gcp/pkg/broker/readiness"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
)

type ReadinessCheckable interface {
	getBrokerCellName() string
	getDataplanePodsRoles() []string
	getGeneration() int64
	getKey() string
	getGenFn(client GenerationQueryServiceClient) getGenClientFn
}

type BrokerReadinessCheck struct {
	b *brokerv1beta1.Broker
}

func ReadinessCheckableForBroker(b *brokerv1beta1.Broker) ReadinessCheckable {
	return &BrokerReadinessCheck{
		b: b,
	}
}

var _ ReadinessCheckable = &BrokerReadinessCheck{}

func (m *BrokerReadinessCheck) getBrokerCellName() string {
	// TODO: change resources.DefaultBrokerCellName to read the brokercelln annotation from the broker object
	return resources.DefaultBrokerCellName
}

func (m *BrokerReadinessCheck) getDataplanePodsRoles() []string {
	return []string{"ingress", "fanout"}
}

func (m *BrokerReadinessCheck) getGeneration() int64 {
	return m.b.Generation
}

func (m *BrokerReadinessCheck) getKey() string {
	return config.KeyFromBroker(m.b).PersistenceString()
}

func (m *BrokerReadinessCheck) getGenFn(client GenerationQueryServiceClient) getGenClientFn {
	return client.GetCellTenantGeneration
}
