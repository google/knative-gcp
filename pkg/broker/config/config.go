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
)

// ReadonlyTargets provides "read" functions for brokers and targets.
type ReadonlyTargets interface {
	// RangeAllTargets ranges over all targets.
	// Do not modify the given Target copy.
	RangeAllTargets(func(*Target) bool)
	// GetTargetByKey returns a target by its trigger key. The format of trigger key is namespace/brokerName/targetName.
	// Do not modify the returned Target copy.
	GetTargetByKey(key TargetKey) (*Target, bool)
	// GetBroker by its key (namespace/name).
	GetGCPAddressableByKey(key GCPCellAddressableKey) (*GcpCellAddressable, bool)
	// RangeGCPCellAddressables ranges over all the GCPCellAddressages.
	// Do not modify the given GcpCellAddressable copy.
	RangeGCPCellAddressables(func(addressable *GcpCellAddressable) bool)
	// Bytes serializes all the targets.
	Bytes() ([]byte, error)
	// String returns the text format of all the targets.
	DebugString() string
	// EqualsBytes checks if the current targets config equals the given
	// targets config in bytes.
	EqualsBytes([]byte) bool
}

// BrokerMutation provides functions to mutate a Broker.
// The changes made via the BrokerMutation must be "committed" altogether.
type BrokerMutation interface {
	// SetID sets the broker ID.
	SetID(id string) BrokerMutation
	// SetAddress sets the broker address.
	SetAddress(address string) BrokerMutation
	// SetDecoupleQueue sets the broker decouple queue.
	SetDecoupleQueue(q *Queue) BrokerMutation
	// SetState sets the broker state.
	SetState(s State) BrokerMutation
	// UpsertTargets upserts Targets to the broker.
	// The targets' namespace and broker will be forced to be
	// the same as the broker's namespace and name.
	UpsertTargets(...*Target) BrokerMutation
	// DeleteTargets targets deletes Targets from the broker.
	DeleteTargets(...*Target) BrokerMutation
	// Delete deletes the broker.
	Delete()
}

// Targets provides "read" and "write" functions for broker targets.
type Targets interface {
	ReadonlyTargets
	// MutateBroker mutates a broker by namespace and name.
	// If the broker doesn't exist, it will be added (unless Delete() is called).
	MutateBroker(namespace, name string, mutate func(BrokerMutation))
}

type GCPCellAddressableKey struct {
	Namespace string
	Name      string
	Type      GcpCellAddressableType
}

func (k GCPCellAddressableKey) PersistenceString() string {
	if k.Type == GcpCellAddressableType_BROKER {
		// For backwards compatibility from when the only type was Broker, Brokers do not embed
		// their type into the string.
		return k.Namespace + "/" + k.Name
	}
	return fmt.Sprintf("%s/%s/%s", k.Type, k.Namespace, k.Name)
}

type TargetKey struct {
	gcpCellAddressableKey GCPCellAddressableKey
	name                  string
}

// BrokerKey returns the key of a broker.
func BrokerKey(namespace, name string) GCPCellAddressableKey {
	return GCPCellAddressableKey{
		Type:      GcpCellAddressableType_BROKER,
		Namespace: namespace,
		Name:      name,
	}
}

// Key returns the target key.
func (x *Target) Key() TargetKey {
	return TargetKey{
		gcpCellAddressableKey: GCPCellAddressableKey{
			Type:      x.GcpCellAddressableType,
			Namespace: x.Namespace,
			Name:      x.GcpCellAddressableName,
		},
		name: x.Name,
	}
}

func (t *TargetKey) GCPCellAddressableKey() GCPCellAddressableKey {
	return t.gcpCellAddressableKey
}

func (t *TargetKey) LogString() string {
	return fmt.Sprintf("%s/%s", t.GCPCellAddressableKey().PersistenceString(), t.name)
}

// Key returns the broker key.
func (x *GcpCellAddressable) Key() GCPCellAddressableKey {
	return GCPCellAddressableKey{
		Type:      x.Type,
		Namespace: x.Namespace,
		Name:      x.Name,
	}
}
