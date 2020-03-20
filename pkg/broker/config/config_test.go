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
	"testing"

	"github.com/golang/protobuf/proto"
)

func TestBaseTargetsRange(t *testing.T) {
	ns1Targets := &NamespacedTargets{
		Names: map[string]*Target{
			"name1": {
				Id:                "uid-1",
				Name:              "name1",
				Namespace:         "ns1",
				FilterAttributes:  map[string]string{"app": "foo"},
				RetryTopic:        "abc",
				RetrySubscription: "abc-sub",
				State:             Target_READY,
			},
			"name2": {
				Id:                "uid-2",
				Name:              "name2",
				Namespace:         "ns1",
				FilterAttributes:  map[string]string{"app": "bar"},
				RetryTopic:        "def",
				RetrySubscription: "def-sub",
				State:             Target_READY,
			},
		},
	}
	ns2Targets := &NamespacedTargets{
		Names: map[string]*Target{
			"name3": {
				Id:                "uid-3",
				Name:              "name3",
				Namespace:         "ns2",
				FilterAttributes:  map[string]string{"app": "bar"},
				RetryTopic:        "ghi",
				RetrySubscription: "ghi-sub",
				State:             Target_UNKNOWN,
			},
			"name4": {
				Id:                "uid-4",
				Name:              "name4",
				Namespace:         "ns2",
				FilterAttributes:  map[string]string{"app": "foo"},
				RetryTopic:        "jkl",
				RetrySubscription: "jkl-sub",
				State:             Target_UNKNOWN,
			},
		},
	}
	// Use a proto message to hold all targets for easy comparison.
	allTargets := &NamespacedTargets{
		Names: map[string]*Target{
			"name1": ns1Targets.GetNames()["name1"],
			"name2": ns1Targets.GetNames()["name2"],
			"name3": ns2Targets.GetNames()["name3"],
			"name4": ns2Targets.GetNames()["name4"],
		},
	}

	targets := &BaseTargets{}
	targets.Internal.Store(&TargetsConfig{
		Namespaces: map[string]*NamespacedTargets{
			"ns1": ns1Targets,
			"ns2": ns2Targets,
		},
	})

	var gotTargets *NamespacedTargets
	targets.RangeNamespace("non-exist", func(t Target) bool {
		if gotTargets == nil {
			gotTargets = &NamespacedTargets{Names: make(map[string]*Target)}
		}
		gotTargets.GetNames()[t.GetName()] = &t
		return true
	})
	if gotTargets != nil {
		t.Errorf("BaseTargets.Range non-existent namespace got=%+v, want nil", gotTargets)
	}

	gotTargets = &NamespacedTargets{Names: make(map[string]*Target)}
	targets.RangeNamespace("ns1", func(t Target) bool {
		gotTargets.GetNames()[t.GetName()] = &t
		return true
	})
	if !proto.Equal(gotTargets, ns1Targets) {
		t.Errorf(`BaseTargets.RangeNamespace("ns1") got=%+v, want=%+v`, gotTargets, ns1Targets)
	}

	gotTargets = &NamespacedTargets{Names: make(map[string]*Target)}
	targets.RangeNamespace("ns2", func(t Target) bool {
		gotTargets.GetNames()[t.GetName()] = &t
		return true
	})
	if !proto.Equal(gotTargets, ns2Targets) {
		t.Errorf(`BaseTargets.RangeNamespace("ns2") got=%+v, want=%+v`, gotTargets, ns2Targets)
	}

	gotTargets = &NamespacedTargets{Names: make(map[string]*Target)}
	targets.Range(func(t Target) bool {
		gotTargets.GetNames()[t.GetName()] = &t
		return true
	})
	if !proto.Equal(gotTargets, allTargets) {
		t.Errorf("BaseTargets.Range got=%+v, want=%+v", gotTargets, allTargets)
	}
}

func TestBaseTargetsBytes(t *testing.T) {
	targets := &BaseTargets{}
	cfg := &TargetsConfig{
		Namespaces: map[string]*NamespacedTargets{
			"ns1": &NamespacedTargets{
				Names: map[string]*Target{
					"name1": {
						Id:                "uid-1",
						Name:              "name1",
						Namespace:         "ns1",
						FilterAttributes:  map[string]string{"app": "foo"},
						RetryTopic:        "abc",
						RetrySubscription: "abc-sub",
						State:             Target_READY,
					},
				},
			},
			"ns2": &NamespacedTargets{
				Names: map[string]*Target{
					"name3": {
						Id:                "uid-3",
						Name:              "name3",
						Namespace:         "ns2",
						FilterAttributes:  map[string]string{"app": "bar"},
						RetryTopic:        "ghi",
						RetrySubscription: "ghi-sub",
						State:             Target_UNKNOWN,
					},
				},
			},
		},
	}
	targets.Internal.Store(cfg)

	wantBytes, err := proto.Marshal(cfg)
	if err != nil {
		t.Fatalf("unexpected error from proto.Marshal: %v", err)
	}

	gotBytes, err := targets.Bytes()
	if err != nil {
		t.Errorf("unexpected error from targets.Byte(): %v", err)
	}
	var gotCfg TargetsConfig
	if err := proto.Unmarshal(gotBytes, &gotCfg); err != nil {
		t.Errorf("unexpected error from proto.Unmarshal: %v", err)
	}
	if !proto.Equal(&gotCfg, cfg) {
		t.Errorf("got unmarshaled targets=%+v, want=%+v", gotCfg, cfg)
	}

	// Test EqualsBytes
	if !targets.EqualsBytes(wantBytes) {
		t.Error("BaseTargets.EqualsBytes() got=false, want=true")
	}
}

func TestBaseTargetsString(t *testing.T) {
	targets := &BaseTargets{}
	cfg := &TargetsConfig{
		Namespaces: map[string]*NamespacedTargets{
			"ns1": &NamespacedTargets{
				Names: map[string]*Target{
					"name1": {
						Id:                "uid-1",
						Name:              "name1",
						Namespace:         "ns1",
						FilterAttributes:  map[string]string{"app": "foo"},
						RetryTopic:        "abc",
						RetrySubscription: "abc-sub",
						State:             Target_READY,
					},
				},
			},
			"ns2": &NamespacedTargets{
				Names: map[string]*Target{
					"name3": {
						Id:                "uid-3",
						Name:              "name3",
						Namespace:         "ns2",
						FilterAttributes:  map[string]string{"app": "bar"},
						RetryTopic:        "ghi",
						RetrySubscription: "ghi-sub",
						State:             Target_UNKNOWN,
					},
				},
			},
		},
	}
	targets.Internal.Store(cfg)

	gotStr := targets.String()
	wantStr := cfg.String()
	if gotStr != wantStr {
		t.Errorf("BaseTargets.String() got=%s, want=%s", gotStr, wantStr)
	}

	// Test EqualsString
	if !targets.EqualsString(wantStr) {
		t.Error("BaseTargets.EqualsString() got=false, want=true")
	}
}
