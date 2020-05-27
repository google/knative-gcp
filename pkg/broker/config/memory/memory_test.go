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

	"github.com/google/knative-gcp/pkg/broker/config"
	"google.golang.org/protobuf/proto"
)

func TestNewEmptyTargets(t *testing.T) {
	v := NewEmptyTargets()
	wantTargets := &config.TargetsConfig{Brokers: make(map[string]*config.Broker)}
	gotTargets := v.(*memoryTargets).Load()
	if !proto.Equal(wantTargets, gotTargets) {
		t.Errorf("NewEmptyTargets have targets got=%+v, want=%+v", gotTargets, wantTargets)
	}
}

func TestUpsertTargetsWithNamespaceBrokerEnforced(t *testing.T) {
	v := NewEmptyTargets()
	wantTarget := &config.Target{
		Name:      "target",
		Namespace: "ns",
		Broker:    "broker",
	}
	v.MutateBroker("ns", "broker", func(bm config.BrokerMutation) {
		bm.UpsertTargets(&config.Target{
			Name:      "target",
			Namespace: "other-namespace",
			Broker:    "other-broker",
		})
	})

	var gotTarget *config.Target
	v.RangeAllTargets(func(t *config.Target) bool {
		gotTarget = t
		return false
	})

	if !proto.Equal(wantTarget, gotTarget) {
		t.Errorf("Upserted target got=%+v, want=%+v", gotTarget, wantTarget)
	}
}

func TestMutateBroker(t *testing.T) {
	val := &config.TargetsConfig{}
	targets := NewTargets(val)

	wantBroker := &config.Broker{
		Id:        "b-uid",
		Address:   "broker.example.com",
		Name:      "broker",
		Namespace: "ns",
		DecoupleQueue: &config.Queue{
			Topic:        "topic",
			Subscription: "sub",
		},
		State: config.State_READY,
	}

	t.Run("create new broker", func(t *testing.T) {
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			m.SetID("b-uid").SetAddress("broker.example.com").SetState(config.State_READY)
			m.SetDecoupleQueue(&config.Queue{
				Topic:        "topic",
				Subscription: "sub",
			})
		})
		assertBroker(t, wantBroker, "ns", "broker", targets)
	})

	t.Run("update broker attribute", func(t *testing.T) {
		wantBroker.Address = "external.broker.example.com"
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			m.SetAddress("external.broker.example.com")
		})
		assertBroker(t, wantBroker, "ns", "broker", targets)
	})

	t1 := &config.Target{
		Id:      "uid-1",
		Address: "consumer1.example.com",
		Broker:  "broker",
		FilterAttributes: map[string]string{
			"app": "foo",
		},
		Name:      "t1",
		Namespace: "ns",
		RetryQueue: &config.Queue{
			Topic:        "topic1",
			Subscription: "sub1",
		},
	}
	t2 := &config.Target{
		Id:      "uid-2",
		Address: "consumer2.example.com",
		Broker:  "broker",
		FilterAttributes: map[string]string{
			"app": "bar",
		},
		Name:      "t2",
		Namespace: "ns",
		RetryQueue: &config.Queue{
			Topic:        "topic2",
			Subscription: "sub2",
		},
	}
	t3 := &config.Target{
		Id:      "uid-3",
		Address: "consumer3.example.com",
		Broker:  "broker",
		FilterAttributes: map[string]string{
			"app": "zzz",
		},
		Name:      "t3",
		Namespace: "ns",
		RetryQueue: &config.Queue{
			Topic:        "topic3",
			Subscription: "sub3",
		},
	}

	t.Run("insert targets", func(t *testing.T) {
		wantBroker.Targets = map[string]*config.Target{
			"t1": t1,
			"t2": t2,
		}
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			// Intentionally call insert twice to verify they can be chained.
			m.UpsertTargets(t1).UpsertTargets(t2)
		})
		assertBroker(t, wantBroker, "ns", "broker", targets)
	})

	t.Run("insert and delete targets", func(t *testing.T) {
		wantBroker.Targets = map[string]*config.Target{
			"t1": t1,
			"t3": t3,
		}
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			// Chain insert and delete.
			m.UpsertTargets(t3).DeleteTargets(t2)
		})
		assertBroker(t, wantBroker, "ns", "broker", targets)
	})

	t.Run("delete and then change broker", func(t *testing.T) {
		wantBroker.Targets = map[string]*config.Target{
			"t1": t1,
			"t2": t2,
		}
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			// Delete should "delete" the broker.
			m.Delete()
			// Then make some changes which should "recreate" the broker.
			m.SetID("b-uid").SetAddress("external.broker.example.com").SetState(config.State_READY)
			m.SetDecoupleQueue(&config.Queue{
				Topic:        "topic",
				Subscription: "sub",
			})
			m.UpsertTargets(t1, t2)
		})
		assertBroker(t, wantBroker, "ns", "broker", targets)
	})

	t.Run("make change then delete broker", func(t *testing.T) {
		targets.MutateBroker("ns", "broker", func(m config.BrokerMutation) {
			m.UpsertTargets(t3).DeleteTargets(t2)
			// Because we delete in the end, the broker should really be deleted
			// no matter what changes have been made.
			m.Delete()
		})
		if _, ok := targets.GetBroker("ns", "broker"); ok {
			t.Error("GetBroker got ok=true, want ok=false")
		}
	})

	t.Run("delete non-existing broker", func(t *testing.T) {
		targets.MutateBroker("ns", "non-existing", func(m config.BrokerMutation) {
			// Just assert it won't panic.
			m.Delete()
		})
	})
}

func assertBroker(t *testing.T, want *config.Broker, namespace, name string, targets config.Targets) {
	t.Helper()
	got, ok := targets.GetBroker(namespace, name)
	if !ok {
		t.Error("GetBroker got ok=false, want ok=true")
	}
	if !proto.Equal(want, got) {
		t.Errorf("GetBroker got broker=%+v, want=%+v", got, want)
	}
}
