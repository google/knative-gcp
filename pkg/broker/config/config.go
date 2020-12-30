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

// ReadonlyTargets provides "read" functions for CellTenants and targets.
type ReadonlyTargets interface {
	// RangeAllTargets ranges over all targets.
	// Do not modify the given Target copy.
	RangeAllTargets(func(*Target) bool)
	// GetTargetByKey returns a target by its trigger key, if it exists.
	// Do not modify the returned Target copy.
	GetTargetByKey(key *TargetKey) (*Target, bool)
	// GetCellTenantByKey returns a CellTenant and its Targets, if it exists.
	// Do not modify the returned CellTenant copy.
	GetCellTenantByKey(key *CellTenantKey) (*CellTenant, bool)
	// RangeCellTenants ranges over all CellTenants.
	// Do not modify the given CellTenant copy.
	RangeCellTenants(func(*CellTenant) bool)
	// Bytes serializes all the targets.
	Bytes() ([]byte, error)
	// DebugString returns the text format of all the targets. It is for _debug_ purposes only. The
	// output format is not guaranteed to be stable and may change at any time.
	DebugString() string
}

// CellTenantMutation provides functions to mutate a CellTenant.
// The changes made via the CellTenantMutation must be "committed" altogether.
type CellTenantMutation interface {
	// SetID sets the CellTenant's ID.
	SetID(id string) CellTenantMutation
	// SetAddress sets the CellTenant's subscriber's address.
	SetAddress(address string) CellTenantMutation
	// SetDecoupleQueue sets the CellTenant's decouple queue.
	SetDecoupleQueue(q *Queue) CellTenantMutation
	// SetState sets the CellTenant's state.
	SetState(s State) CellTenantMutation
	// UpsertTargets upserts Targets to the CellTenant.
	// The targets' namespace, CellTenantType, and CellTenantName will be set to the CellTenant's
	// value.
	UpsertTargets(...*Target) CellTenantMutation
	// DeleteTargets targets deletes Targets from the CellTenant.
	DeleteTargets(...*Target) CellTenantMutation
	// Delete deletes the CellTenant.
	Delete()
}

// Targets provides "read" and "write" functions for BrokerCell targets.
type Targets interface {
	ReadonlyTargets
	// MutateCellTenant mutates a CellTenant by its key.
	// If the CellTenant doesn't exist, it will be added (unless Delete() is called).
	MutateCellTenant(key *CellTenantKey, mutate func(CellTenantMutation))
}
