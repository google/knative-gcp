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

	"github.com/golang/protobuf/proto"
)

// ReadOnlyTargets provides "read" functions for broker targets.
type ReadOnlyTargets interface {
	// RangeNamespace ranges over targets in the given namespace.
	RangeNamespace(namespace string, f func(t Target) bool)
	// Range ranges over all targets.
	Range(f func(t Target) bool)
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

// Targets provides "read" and "write" functions for broker targets.
type Targets interface {
	ReadOnlyTargets

	// Insert adds the given targets to the current list.
	Insert(...Target) Targets
	// Delete removes the give targets from the current list.
	Delete(...Target) Targets
}

// BaseTargets provide a common field to store the targets config and
// implements common functionalities.
type BaseTargets struct {
	Internal atomic.Value
}

// RangeNamespace ranges over targets in the given namespace.
func (bt *BaseTargets) RangeNamespace(namespace string, f func(Target) bool) {
	cfg := bt.Internal.Load().(*TargetsConfig)
	if cfg == nil {
		return
	}
	if _, ok := cfg.GetNamespaces()[namespace]; !ok {
		return
	}

	for _, target := range cfg.GetNamespaces()[namespace].GetNames() {
		if c := f(*target); !c {
			break
		}
	}
}

// Range ranges over all targets.
func (bt *BaseTargets) Range(f func(Target) bool) {
	cfg := bt.Internal.Load().(*TargetsConfig)
	for _, nt := range cfg.GetNamespaces() {
		for _, target := range nt.GetNames() {
			if c := f(*target); !c {
				break
			}
		}
	}
}

// Bytes serializes all the targets.
func (bt *BaseTargets) Bytes() ([]byte, error) {
	cfg := bt.Internal.Load().(*TargetsConfig)
	return proto.Marshal(cfg)
}

// String returns the text format of all the targets.
func (bt *BaseTargets) String() string {
	cfg := bt.Internal.Load().(*TargetsConfig)
	return cfg.String()
}

// EqualsBytes checks if the current targets config equals the given
// targets config in bytes.
func (bt *BaseTargets) EqualsBytes(b []byte) bool {
	self := bt.Internal.Load().(*TargetsConfig)
	var other TargetsConfig
	if err := proto.Unmarshal(b, &other); err != nil {
		return false
	}
	return proto.Equal(self, &other)
}

// EqualsString checks if the current targets config equals the given
// targets config in string.
func (bt *BaseTargets) EqualsString(s string) bool {
	self := bt.Internal.Load().(*TargetsConfig)
	var other TargetsConfig
	if err := proto.UnmarshalText(s, &other); err != nil {
		return false
	}
	return proto.Equal(self, &other)
}
