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

package resources

// This file contains functions that construct knative-gcp resources.

import (
	"github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerV1Beta1Option enables further configuration of a Broker.
type BrokerV1Beta1Option func(*v1beta1.Broker)

// WithBrokerClassForBrokerV1Beta1 returns a function that adds a brokerClass
// annotation to the given Broker.
func WithBrokerClassForBrokerV1Beta1(brokerClass string) BrokerV1Beta1Option {
	return func(b *v1beta1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations["eventing.knative.dev/broker.class"] = brokerClass
		b.SetAnnotations(annotations)
	}
}

// Broker returns a Broker.
func BrokerV1Beta1(name string, options ...BrokerV1Beta1Option) *v1beta1.Broker {
	broker := &v1beta1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(broker)
	}
	return broker
}
