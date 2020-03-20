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
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

// Targets implements config.Targets with data stored in memory.
type Targets struct {
	config.BaseTargets
}

var _ config.Targets = (*Targets)(nil)

// NewTargetsFromBytes initializes the in memory targets from bytes.
func NewTargetsFromBytes(b []byte) (config.Targets, error) {
	t := &Targets{
		BaseTargets: config.BaseTargets{},
	}
	pb := config.TargetsConfig{}
	if err := proto.Unmarshal(b, &pb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bytes to TargetsConfig: %w", err)
	}
	t.Internal.Store(&pb)
	return t, nil
}

// Union adds the given targets.
func (t *Targets) Union(targets ...config.Target) config.Targets {
	cfg := t.Internal.Load().(*config.TargetsConfig)
	if cfg.GetNamespaces() == nil {
		cfg.Namespaces = make(map[string]*config.NamespacedTargets)
	}
	for _, target := range targets {
		local := target
		if _, ok := cfg.GetNamespaces()[target.GetNamespace()]; ok {
			cfg.GetNamespaces()[target.GetNamespace()].GetNames()[target.GetName()] = &local
		} else {
			cfg.GetNamespaces()[target.GetNamespace()] = &config.NamespacedTargets{
				Names: map[string]*config.Target{
					target.GetName(): &local,
				},
			}
		}
	}
	t.Internal.Store(cfg)
	return t
}

// Except removes the give targets.
func (t *Targets) Except(targets ...config.Target) config.Targets {
	cfg := t.Internal.Load().(*config.TargetsConfig)
	for _, target := range targets {
		if _, ok := cfg.GetNamespaces()[target.GetNamespace()]; !ok {
			continue
		}
		delete(cfg.GetNamespaces()[target.GetNamespace()].GetNames(), target.GetName())
		if len(cfg.GetNamespaces()[target.GetNamespace()].GetNames()) == 0 {
			// If no more targets in the namespace, also delete the namespace.
			delete(cfg.GetNamespaces(), target.GetNamespace())
		}
	}
	t.Internal.Store(cfg)
	return t
}
