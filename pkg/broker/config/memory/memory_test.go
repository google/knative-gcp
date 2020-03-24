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
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

var (
	ns1Targets = []*config.Target{
		{
			Id:                "uid-1",
			Name:              "name1",
			Namespace:         "ns1",
			FilterAttributes:  map[string]string{"app": "foo"},
			RetryTopic:        "abc",
			RetrySubscription: "abc-sub",
			State:             config.Target_READY,
		},
		{
			Id:                "uid-2",
			Name:              "name2",
			Namespace:         "ns1",
			FilterAttributes:  map[string]string{"app": "bar"},
			RetryTopic:        "def",
			RetrySubscription: "def-sub",
			State:             config.Target_READY,
		},
	}
	ns2Targets = []*config.Target{
		{
			Id:                "uid-3",
			Name:              "name3",
			Namespace:         "ns2",
			FilterAttributes:  map[string]string{"app": "bar"},
			RetryTopic:        "ghi",
			RetrySubscription: "ghi-sub",
			State:             config.Target_UNKNOWN,
		},
		{
			Id:                "uid-4",
			Name:              "name4",
			Namespace:         "ns2",
			FilterAttributes:  map[string]string{"app": "foo"},
			RetryTopic:        "jkl",
			RetrySubscription: "jkl-sub",
			State:             config.Target_UNKNOWN,
		},
	}
)

func TestInsert(t *testing.T) {
	wantTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name1": ns1Targets[0],
					"name2": ns1Targets[1],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name3": ns2Targets[0],
					"name4": ns2Targets[1],
				},
			},
		},
	}

	initTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name1": ns1Targets[0],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name3": ns2Targets[0],
				},
			},
		},
	}
	b, _ := proto.Marshal(initTargets)
	targets, err := NewTargetsFromBytes(b)
	if err != nil {
		t.Fatalf("unexpected from NewTargetsFromBytes: %v", err)
	}

	targets = targets.Insert(*ns1Targets[1], *ns2Targets[1])
	gotTargets := targets.(*Targets).Internal.Load().(*config.TargetsConfig)
	if !proto.Equal(wantTargets, gotTargets) {
		t.Errorf("targets after insert got=%+v,want%+v", gotTargets, wantTargets)
	}
}

func TestDelete(t *testing.T) {
	wantTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name1": ns1Targets[0],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name3": ns2Targets[0],
				},
			},
		},
	}

	initTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name1": ns1Targets[0],
					"name2": ns1Targets[1],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name3": ns2Targets[0],
					"name4": ns2Targets[1],
				},
			},
		},
	}
	b, _ := proto.Marshal(initTargets)
	targets, err := NewTargetsFromBytes(b)
	if err != nil {
		t.Fatalf("unexpected from NewTargetsFromBytes: %v", err)
	}

	targets = targets.Delete(*ns1Targets[1], *ns2Targets[1])
	gotTargets := targets.(*Targets).Internal.Load().(*config.TargetsConfig)
	if !proto.Equal(wantTargets, gotTargets) {
		t.Errorf("targets after delete got=%+v, want=%+v", gotTargets, wantTargets)
	}
}

func TestInsertDelete(t *testing.T) {
	wantTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name2": ns1Targets[1],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name4": ns2Targets[1],
				},
			},
		},
	}
	initTargets := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": {
				Names: map[string]*config.Target{
					"name1": ns1Targets[0],
				},
			},
			"ns2": {
				Names: map[string]*config.Target{
					"name3": ns2Targets[0],
				},
			},
		},
	}
	b, _ := proto.Marshal(initTargets)
	targets, err := NewTargetsFromBytes(b)
	if err != nil {
		t.Fatalf("unexpected from NewTargetsFromBytes: %v", err)
	}

	targets = targets.Insert(*ns1Targets[1], *ns2Targets[1]).Delete(*ns1Targets[0], *ns2Targets[0])
	gotTargets := targets.(*Targets).Internal.Load().(*config.TargetsConfig)
	if !proto.Equal(wantTargets, gotTargets) {
		t.Errorf("targets after insert and delete got=%+v, want=%+v", gotTargets, wantTargets)
	}
}
