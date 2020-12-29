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
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
)

// BrokerKey uniquely identifies a single Broker, at a given point in time.
type BrokerKey struct {
	namespace string
	name      string
}

// PersistenceString is the string that is persisted as the key for this Broker in the protobuf. It
// is stable and can only change if all existing usage locations are made backwards compatible,
// supporting _both_ the old and the new format, for at least one release.
func (k *BrokerKey) PersistenceString() string {
	return k.namespace + "/" + k.name
}

// String creates a human readable version of this key. It is for debug purposes only. It is free to
// change at any time.
func (k *BrokerKey) String() string {
	// Note that this is explicitly different than the PersistenceString, so that we don't
	// accidentally use String(), rather than PersistenceString().
	return k.namespace + "//" + k.name
}

// CreateEmptyBroker creates an empty Broker that corresponds to this BrokerKey. It is empty except
// for the portions known about by the BrokerKey.
func (k *BrokerKey) CreateEmptyBroker() *Broker {
	return &Broker{
		Namespace: k.namespace,
		Name:      k.name,
	}
}

// Key returns the BrokerKey for this Broker.
func (x *Broker) Key() *BrokerKey {
	return &BrokerKey{
		namespace: x.Namespace,
		name:      x.Name,
	}
}

// TargetKey uniquely identifies a single Target, at a given point in time.
type TargetKey struct {
	brokerKey BrokerKey
	name      string
}

func (k *TargetKey) ParentKey() *BrokerKey {
	return &k.brokerKey
}

// String creates a human readable version of this key. It is for debug purposes only. It is free to
// change at any time.
func (k *TargetKey) String() string {
	// Note that this is explicitly different than the PersistenceString, so that we don't
	// accidentally use String(), rather than PersistenceString().
	return k.brokerKey.String() + "//" + k.name
}

// Key returns the TargetKey for this Target.
func (x *Target) Key() *TargetKey {
	return &TargetKey{
		brokerKey: BrokerKey{
			namespace: x.Namespace,
			name:      x.Broker,
		},
		name: x.Name,
	}
}

// KeyFromBroker creates a BrokerKey from a K8s Broker object.
func KeyFromBroker(b *brokerv1beta1.Broker) *BrokerKey {
	return &BrokerKey{
		namespace: b.Namespace,
		name:      b.Name,
	}
}

// TestOnlyBrokerKey returns the key of a broker.
func TestOnlyBrokerKey(namespace, name string) *BrokerKey {
	return &BrokerKey{
		namespace: namespace,
		name:      name,
	}
}
