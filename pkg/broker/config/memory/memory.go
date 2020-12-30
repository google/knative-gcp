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

var _ config.CellTenantMutation = (*cellTenantMutation)(nil)

type cellTenantMutation struct {
	b      *config.CellTenant
	delete bool
}

func (m *cellTenantMutation) SetID(id string) config.CellTenantMutation {
	m.delete = false
	m.b.Id = id
	return m
}

func (m *cellTenantMutation) SetAddress(address string) config.CellTenantMutation {
	m.delete = false
	m.b.Address = address
	return m
}

func (m *cellTenantMutation) SetDecoupleQueue(q *config.Queue) config.CellTenantMutation {
	m.delete = false
	m.b.DecoupleQueue = q
	return m
}

func (m *cellTenantMutation) SetState(s config.State) config.CellTenantMutation {
	m.delete = false
	m.b.State = s
	return m
}

func (m *cellTenantMutation) UpsertTargets(targets ...*config.Target) config.CellTenantMutation {
	m.delete = false
	if m.b.Targets == nil {
		m.b.Targets = make(map[string]*config.Target)
	}
	for _, t := range targets {
		t.Namespace = m.b.Namespace
		t.CellTenantType = m.b.Type
		t.CellTenantName = m.b.Name
		m.b.Targets[t.Name] = t
	}
	return m
}

func (m *cellTenantMutation) DeleteTargets(targets ...*config.Target) config.CellTenantMutation {
	m.delete = false
	for _, t := range targets {
		delete(m.b.Targets, t.Name)
	}
	return m
}

func (m *cellTenantMutation) Delete() {
	// Calling delete will "reset" the broker under mutation instantly.
	m.delete = true
	m.b = &config.CellTenant{
		Type:      m.b.Type,
		Name:      m.b.Name,
		Namespace: m.b.Namespace,
	}
}

type memoryTargets struct {
	config.CachedTargets
	mux sync.Mutex
}

var _ config.Targets = (*memoryTargets)(nil)

// NewEmptyTargets returns an empty mutable Targets in memory.
func NewEmptyTargets() config.Targets {
	return NewTargets(&config.TargetsConfig{CellTenants: make(map[string]*config.CellTenant)})
}

// NewTargets returns a new mutable Targets in memory.
func NewTargets(pb *config.TargetsConfig) config.Targets {
	m := &memoryTargets{mux: sync.Mutex{}}
	m.Store(pb)
	return m
}

// MutateCellTenant mutates a CellTenant by its key.
// If the CellTenant doesn't exist, it will be added (unless Delete() is called).
// This function is thread-safe.
func (m *memoryTargets) MutateCellTenant(key *config.CellTenantKey, mutate func(config.CellTenantMutation)) {
	// Sync writes.
	m.mux.Lock()
	defer m.mux.Unlock()

	b := key.CreateEmptyCellTenant()
	var newVal *config.TargetsConfig
	val := m.Load()
	if val != nil {
		// Don't modify the existing copy because it
		// will break the atomic store/load.
		newVal = proto.Clone(val).(*config.TargetsConfig)
	} else {
		newVal = &config.TargetsConfig{}
	}

	if newVal.CellTenants != nil {
		if existing, ok := newVal.CellTenants[key.PersistenceString()]; ok {
			b = existing
		}
	}

	// The mutation will work on a copy of the data.
	mutation := &cellTenantMutation{b: b}
	mutate(mutation)

	if mutation.delete {
		delete(newVal.CellTenants, key.PersistenceString())
	} else {
		if newVal.CellTenants == nil {
			newVal.CellTenants = make(map[string]*config.CellTenant)
		}
		newVal.CellTenants[key.PersistenceString()] = mutation.b
	}

	// Update the atomic value to be the copy.
	// This works like a commit.
	m.Store(newVal)
}
