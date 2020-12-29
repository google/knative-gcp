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

package config

import (
	"fmt"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"

	"k8s.io/apimachinery/pkg/util/validation"
)

// BrokerKey uniquely identifies a single Broker, at a given point in time.
type BrokerKey struct {
	namespace string
	name      string
}

// Key returns the BrokerKey for this Broker.
func (x *Broker) Key() BrokerKey {
	return BrokerKey{
		namespace: x.Namespace,
		name:      x.Name,
	}
}

// KeyFromBroker creates a BrokerKey from a K8s Broker object.
func KeyFromBroker(b *brokerv1beta1.Broker) BrokerKey {
	return BrokerKey{
		namespace: b.Namespace,
		name:      b.Name,
	}
}

func validateNamespace(ns string) error {
	errs := validation.IsDNS1123Label(ns)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid namespace %q, %v", ns, errs)
}

func validateName(name string) error {
	errs := validation.IsDNS1123Label(name)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid name %q, %v", name, errs)
}

func (k *BrokerKey) CreateEmptyBroker() *Broker {
	return &Broker{
		Namespace: k.namespace,
		Name:      k.name,
	}
}

// TestOnlyBrokerKey returns the key of a broker.
func TestOnlyBrokerKey(namespace, name string) string {
	return namespace + "/" + name
}
