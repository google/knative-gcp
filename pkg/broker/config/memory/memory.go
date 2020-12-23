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

	"github.com/google/knative-gcp/pkg/broker/config"
	"google.golang.org/protobuf/proto"
)

var _ config.GCPCellAddressableMutation = (*gcpCellAddressableMutation)(nil)

type gcpCellAddressableMutation struct {
	b      *config.GcpCellAddressable
	delete bool
}

func (m *gcpCellAddressableMutation) SetID(id string) config.GCPCellAddressableMutation {
	m.delete = false
	m.b.Id = id
	return m
}

func (m *gcpCellAddressableMutation) SetAddress(address string) config.GCPCellAddressableMutation {
	m.delete = false
	m.b.Address = address
	return m
}

func (m *gcpCellAddressableMutation) SetDecoupleQueue(q *config.Queue) config.GCPCellAddressableMutation {
	m.delete = false
	m.b.DecoupleQueue = q
	return m
}

func (m *gcpCellAddressableMutation) SetState(s config.State) config.GCPCellAddressableMutation {
	m.delete = false
	m.b.State = s
	return m
}

func (m *gcpCellAddressableMutation) UpsertTargets(targets ...*config.Target) config.GCPCellAddressableMutation {
	m.delete = false
	if m.b.Targets == nil {
		m.b.Targets = make(map[string]*config.Target)
	}
	for _, t := range targets {
		t.Namespace = m.b.Namespace
		t.GcpCellAddressableName = m.b.Name
		m.b.Targets[t.Name] = t
	}
	return m
}

func (m *gcpCellAddressableMutation) DeleteTargets(targets ...*config.Target) config.GCPCellAddressableMutation {
	m.delete = false
	for _, t := range targets {
		delete(m.b.Targets, t.Name)
	}
	return m
}

func (m *gcpCellAddressableMutation) Delete() {
	// Calling delete will "reset" the GCPCellAddressable under mutation instantly.
	m.delete = true
	m.b = &config.GcpCellAddressable{Name: m.b.Name, Namespace: m.b.Namespace}
}

type memoryTargets struct {
	config.CachedTargets
	mux sync.Mutex
}

var _ config.Targets = (*memoryTargets)(nil)

// NewEmptyTargets returns an empty mutable Targets in memory.
func NewEmptyTargets() config.Targets {
	return NewTargets(&config.TargetsConfig{
		GcpCellAddressables: make(map[string]*config.GcpCellAddressable),
	})
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
func (m *memoryTargets) MutateGCPCellAddressable(key config.GCPCellAddressableKey, mutate func(config.GCPCellAddressableMutation)) {
	// Sync writes.
	m.mux.Lock()
	defer m.mux.Unlock()

	var newVal *config.TargetsConfig
	val := m.Load()
	if val != nil {
		// Don't modify the existing copy because it
		// will break the atomic store/load.
		newVal = proto.Clone(val).(*config.TargetsConfig)
	} else {
		newVal = &config.TargetsConfig{}
	}

	b := key.CreateEmptyGCPCellAddressable()
	if newVal.GcpCellAddressables != nil {
		if existing, ok := newVal.GcpCellAddressables[key.PersistenceString()]; ok {
			b = existing
		}
	}

	// The mutation will work on a copy of the data.
	mutation := &gcpCellAddressableMutation{b: b}
	mutate(mutation)

	if mutation.delete {
		delete(newVal.GcpCellAddressables, key.PersistenceString())
	} else {
		if newVal.GcpCellAddressables == nil {
			newVal.GcpCellAddressables = make(map[string]*config.GcpCellAddressable)
		}
		newVal.GcpCellAddressables[key.PersistenceString()] = mutation.b
	}

	// Update the atomic value to be the copy.
	// This works like a commit.
	m.Store(newVal)
}
