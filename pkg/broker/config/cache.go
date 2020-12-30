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
	"sync/atomic"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// CachedTargets provides a in-memory cached copy of targets.
type CachedTargets struct {
	Value atomic.Value
}

var _ ReadonlyTargets = (*CachedTargets)(nil)

// Store atomically stores a TargetsConfig.
func (ct *CachedTargets) Store(t *TargetsConfig) {
	ct.Value.Store(t)
}

// Load atomically loads a stored TargetsConfig.
// If there was no TargetsConfig stored, nil will be returned.
func (ct *CachedTargets) Load() *TargetsConfig {
	tc := ct.Value.Load().(*TargetsConfig)

	// To support downgrades, we need to pretend entries without a CellTenantType are actually
	// Broker typed. That way, if the newer BrokerCell code is running, but has an older ConfigMap,
	// it doesn't think that every CellTenant is of unknown type.
	// TODO Remove after release 0.22.
	for _, t := range tc.CellTenants {
		if t.Type == CellTenantType_UNKNOWN_CELL_TENANT_TYPE {
			t.Type = CellTenantType_BROKER
			for _, tt := range t.Targets {
				tt.CellTenantType = CellTenantType_BROKER
			}
		}
	}

	return tc
}

// RangeAllTargets ranges over all targets.
// Do not modify the given Target copy.
func (ct *CachedTargets) RangeAllTargets(f func(*Target) bool) {
	val := ct.Load()
	if val == nil {
		return
	}
	for _, b := range val.CellTenants {
		for _, t := range b.Targets {
			if c := f(t); !c {
				return
			}
		}
	}
}

// GetTargetByKey returns a target by its trigger key. The format of trigger key is namespace/brokerName/targetName.
// Do not modify the returned Target copy.
func (ct *CachedTargets) GetTargetByKey(key *TargetKey) (*Target, bool) {
	broker, ok := ct.GetCellTenantByKey(key.ParentKey())
	if !ok {
		return nil, false
	}
	t, ok := broker.Targets[key.name]
	return t, ok
}

// GetBrokerByKey returns a broker and its targets if it exists.
// Do not modify the returned Broker copy.
func (ct *CachedTargets) GetCellTenantByKey(key *CellTenantKey) (*CellTenant, bool) {
	val := ct.Load()
	if val == nil || val.CellTenants == nil {
		return nil, false
	}
	b, ok := val.CellTenants[key.PersistenceString()]
	return b, ok
}

// RangeBrokers ranges over all brokers.
// Do not modify the given Broker copy.
func (ct *CachedTargets) RangeCellTenants(f func(*CellTenant) bool) {
	val := ct.Load()
	if val == nil {
		return
	}
	for _, b := range val.CellTenants {
		if c := f(b); !c {
			break
		}
	}
}

// Bytes serializes all the targets.
func (ct *CachedTargets) Bytes() ([]byte, error) {
	val := ct.Load()
	return proto.Marshal(val)
}

// DebugString returns the text format of all the targets. It is for _debug_ purposes only. The
// output format is not guaranteed to be stable and may change at any time.
func (ct *CachedTargets) DebugString() string {
	val := ct.Load()
	return prototext.MarshalOptions{
		Multiline: true,
		Indent:    "\t",
	}.Format(val)
}
