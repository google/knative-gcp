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

package memory

import (
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

type brokerMutation struct {
	b      *config.Broker
	delete bool
}

func (m *brokerMutation) SetID(id string) config.BrokerMutation {
	m.b.Id = id
	return m
}

func (m *brokerMutation) SetAddress(address string) config.BrokerMutation {
	m.b.Address = address
	return m
}

func (m *brokerMutation) SetDecoupleQueue(q *config.Queue) config.BrokerMutation {
	m.b.DecoupleQueue = q
	return m
}

func (m *brokerMutation) SetState(s config.State) config.BrokerMutation {
	m.b.State = s
	return m
}

func (m *brokerMutation) UpsertTargets(targets ...*config.Target) config.BrokerMutation {
	if m.b.Targets == nil {
		m.b.Targets = make(map[string]*config.Target)
	}
	for _, t := range targets {
		t.Namespace = m.b.Namespace
		t.Broker = m.b.Name
		m.b.Targets[t.Name] = t
	}
	return m
}

func (m *brokerMutation) DeleteTargets(targets ...*config.Target) config.BrokerMutation {
	for _, t := range targets {
		delete(m.b.Targets, t.Name)
	}
	return m
}

func (m *brokerMutation) Delete() {
	m.delete = true
}

type memoryTargets struct {
	config.CachedTargets
	mux sync.Mutex
}

var _ config.Targets = (*memoryTargets)(nil)

// NewEmptyTargets returns an empty mutable Targets in memory.
func NewEmptyTargets() config.Targets {
	return NewTargets(&config.TargetsConfig{Brokers: make(map[string]*config.Broker)})
}

// NewTargets returns a new mutable Targets in memory.
func NewTargets(pb *config.TargetsConfig) config.Targets {
	m := &memoryTargets{mux: sync.Mutex{}}
	m.Store(pb)
	return m
}

// MutateBroker mutates a broker by namespace and name.
// If the broker doesn't exist, it will be added (unless Delete() is called).
// This function is thread-safe.
func (m *memoryTargets) MutateBroker(namespace, name string, mutate func(config.BrokerMutation)) {
	// Sync writes.
	m.mux.Lock()
	defer m.mux.Unlock()

	b := &config.Broker{Name: name, Namespace: namespace}
	var newVal *config.TargetsConfig
	val := m.Load()
	if val != nil {
		// Don't modify the existing copy because it
		// will break the atomic store/load.
		newVal = proto.Clone(val).(*config.TargetsConfig)
	} else {
		newVal = &config.TargetsConfig{}
	}

	if newVal.Brokers != nil {
		if existing, ok := newVal.Brokers[config.BrokerKey(namespace, name)]; ok {
			b = existing
		}
	}

	// The mutation will work on a copy of the data.
	mutation := &brokerMutation{b: b}
	mutate(mutation)

	if mutation.delete {
		delete(newVal.Brokers, config.BrokerKey(namespace, name))
	} else {
		if newVal.Brokers == nil {
			newVal.Brokers = make(map[string]*config.Broker)
		}
		newVal.Brokers[config.BrokerKey(namespace, name)] = b
	}

	// Update the atomic value to be the copy.
	// This works like a commit.
	m.Store(newVal)
}
