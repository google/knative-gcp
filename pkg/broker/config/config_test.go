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
	"github.com/google/go-cmp/cmp"
)

func TestBaseTargetsRange(t *testing.T) {
	ns1Targets := []*Target{
		{
			Id:                "uid-1",
			Name:              "name1",
			Namespace:         "ns1",
			FilterAttributes:  map[string]string{"app": "foo"},
			RetryTopic:        "abc",
			RetrySubscription: "abc-sub",
			State:             Target_READY,
		},
		{
			Id:                "uid-2",
			Name:              "name2",
			Namespace:         "ns1",
			FilterAttributes:  map[string]string{"app": "bar"},
			RetryTopic:        "def",
			RetrySubscription: "def-sub",
			State:             Target_READY,
		},
	}
	ns2Targets := []*Target{
		{
			Id:                "uid-3",
			Name:              "name3",
			Namespace:         "ns2",
			FilterAttributes:  map[string]string{"app": "bar"},
			RetryTopic:        "ghi",
			RetrySubscription: "ghi-sub",
			State:             Target_UNKNOWN,
		},
		{
			Id:                "uid-4",
			Name:              "name4",
			Namespace:         "ns2",
			FilterAttributes:  map[string]string{"app": "foo"},
			RetryTopic:        "jkl",
			RetrySubscription: "jkl-sub",
			State:             Target_UNKNOWN,
		},
	}
	var allTargets []*Target
	allTargets = append(allTargets, ns1Targets...)
	allTargets = append(allTargets, ns2Targets...)

	targets := &BaseTargets{}
	targets.Internal.Store(&TargetsConfig{
		Namespaces: map[string]*NamespacedTargets{
			"ns1": &NamespacedTargets{
				Names: map[string]*Target{
					"name1": ns1Targets[0],
					"name2": ns1Targets[1],
				},
			},
			"ns2": &NamespacedTargets{
				Names: map[string]*Target{
					"name3": ns2Targets[0],
					"name4": ns2Targets[1],
				},
			},
		},
	})

	var gotTargets []*Target
	targets.RangeNamespace("non-exist", func(t Target) bool {
		gotTargets = append(gotTargets, &t)
		return true
	})
	if gotTargets != nil {
		t.Errorf("targets range non-existent namespace got targets %+v want nil", gotTargets)
	}

	gotTargets = nil
	targets.RangeNamespace("ns1", func(t Target) bool {
		gotTargets = append(gotTargets, &t)
		return true
	})
	cmpTargets(t, ns1Targets, gotTargets)

	gotTargets = nil
	targets.RangeNamespace("ns2", func(t Target) bool {
		gotTargets = append(gotTargets, &t)
		return true
	})
	cmpTargets(t, ns2Targets, gotTargets)

	gotTargets = nil
	targets.Range(func(t Target) bool {
		gotTargets = append(gotTargets, &t)
		return true
	})
	cmpTargets(t, allTargets, gotTargets)
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

	wantBytes, _ := proto.Marshal(cfg)
	gotBytes, err := targets.Bytes()
	if err != nil {
		t.Fatalf("unexpected error from targets.Bytes(): %v", err)
	}
	if diff := cmp.Diff(wantBytes, gotBytes); diff != "" {
		t.Errorf("targets.Bytes() (-want,+got): %v", diff)
	}
}

func cmpTargets(t *testing.T, t1, t2 []*Target) {
	t.Helper()
	if len(t1) != len(t2) {
		t.Errorf("len(t1) = %d, len(t2) = %d", len(t1), len(t2))
	}
	for i, v := range t1 {
		if !proto.Equal(v, t2[i]) {
			t.Errorf("t1[%d] != t2[%d], %v %v", i, i, v, t2[i])
		}
	}
}
