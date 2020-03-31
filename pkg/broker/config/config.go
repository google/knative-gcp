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

// ReadonlyTargets provides "read" functions for brokers and targets.
type ReadonlyTargets interface {
	// RangeAllTargets ranges over all targets.
	// Do not modify the given Target copy.
	RangeAllTargets(func(*Target) bool)
	// GetBroker returns a broker and its targets if it exists.
	// Do not modify the returned Broker copy.
	GetBroker(namespace, name string) (*Broker, bool)
	// RangeBrokers ranges over all brokers.
	// Do not modify the given Broker copy.
	RangeBrokers(func(*Broker) bool)
	// Bytes serializes all the targets.
	Bytes() ([]byte, error)
	// String returns the text format of all the targets.
	String() string
	// EqualsBytes checks if the current targets config equals the given
	// targets config in bytes.
	EqualsBytes([]byte) bool
	// EqualsString checks if the current targets config equals the given
	// targets config in string.
	EqualsString(string) bool
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
	// InsertTargets inserts Targets to the broker.
	InsertTargets(...*Target) BrokerMutation
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

// BrokerKey returns the key of a broker.
func BrokerKey(namespace, name string) string {
	return namespace + "/" + name
}
