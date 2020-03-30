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

func TestCachedTargetsRange(t *testing.T) {
	t1 := &Target{
		Id:               "uid-1",
		Name:             "name1",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "abc",
			Subscription: "abc-sub",
		},
		State: State_READY,
	}
	t2 := &Target{
		Id:               "uid-2",
		Name:             "name2",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "def",
			Subscription: "def-sub",
		},
		State: State_READY,
	}
	t3 := &Target{
		Id:               "uid-3",
		Name:             "name3",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "ghi",
			Subscription: "ghi-sub",
		},
		State: State_UNKNOWN,
	}
	t4 := &Target{
		Id:               "uid-4",
		Name:             "name4",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "jkl",
			Subscription: "jkl-sub",
		},
		State: State_UNKNOWN,
	}
	b1 := &Broker{
		Id:        "b-uid-1",
		Address:   "broker1.ns1.example.com",
		Name:      "broker1",
		Namespace: "ns1",
		DecoupleQueue: &Queue{
			Topic:        "topic1",
			Subscription: "sub1",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name1": t1,
			"name2": t2,
		},
	}
	b2 := &Broker{
		Id:        "b-uid-2",
		Address:   "broker2.ns2.example.com",
		Name:      "broker2",
		Namespace: "ns2",
		DecoupleQueue: &Queue{
			Topic:        "topic2",
			Subscription: "sub2",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name3": t3,
			"name4": t4,
		},
	}
	val := &TargetsConfig{
		Brokers: map[string]*Broker{
			"ns1/broker1": b1,
			"ns2/broker2": b2,
		},
	}

	targets := &CachedTargets{}
	targets.Store(val)

	t.Run("range all targets", func(t *testing.T) {
		wantTargets := map[string]*Target{
			"name1": t1,
			"name2": t2,
			"name3": t3,
			"name4": t4,
		}
		gotTargets := make(map[string]*Target)
		targets.RangeAllTargets(func(t *Target) bool {
			gotTargets[t.Name] = t
			return true
		})
		if diff := cmp.Diff(wantTargets, gotTargets); diff != "" {
			t.Errorf("RangeAllTargets (-want,+got): %v", diff)
		}
	})

	t.Run("range brokers", func(t *testing.T) {
		gotBrokers := make(map[string]*Broker)
		targets.RangeBrokers(func(b *Broker) bool {
			gotBrokers[BrokerKey(b.Namespace, b.Name)] = b
			return true
		})
		if diff := cmp.Diff(val.Brokers, gotBrokers); diff != "" {
			t.Errorf("RangeBrokers (-want,+got): %v", diff)
		}
	})

	t.Run("get individual broker", func(t *testing.T) {
		gotBroker, ok := targets.GetBroker("ns", "non-existing")
		if ok {
			t.Error("get non-existing broker got ok=true, want ok=false")
		}
		gotBroker, ok = targets.GetBroker(b1.Namespace, b1.Name)
		if !ok {
			t.Error("get existing broker got ok=false, want ok=true")
		}
		if !proto.Equal(b1, gotBroker) {
			t.Errorf("get existing broker got=%+v, want=%+v", gotBroker, b1)
		}
		gotBroker, ok = targets.GetBroker(b2.Namespace, b2.Name)
		if !ok {
			t.Error("get existing broker got ok=false, want ok=true")
		}
		if !proto.Equal(b2, gotBroker) {
			t.Errorf("get existing broker got=%+v, want=%+v", gotBroker, b1)
		}
	})
}

func TestCachedTargetsBytes(t *testing.T) {
	t1 := &Target{
		Id:               "uid-1",
		Name:             "name1",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "abc",
			Subscription: "abc-sub",
		},
		State: State_READY,
	}
	t2 := &Target{
		Id:               "uid-2",
		Name:             "name2",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "def",
			Subscription: "def-sub",
		},
		State: State_READY,
	}
	t3 := &Target{
		Id:               "uid-3",
		Name:             "name3",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "ghi",
			Subscription: "ghi-sub",
		},
		State: State_UNKNOWN,
	}
	t4 := &Target{
		Id:               "uid-4",
		Name:             "name4",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "jkl",
			Subscription: "jkl-sub",
		},
		State: State_UNKNOWN,
	}
	b1 := &Broker{
		Id:        "b-uid-1",
		Address:   "broker1.ns1.example.com",
		Name:      "broker1",
		Namespace: "ns1",
		DecoupleQueue: &Queue{
			Topic:        "topic1",
			Subscription: "sub1",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name1": t1,
			"name2": t2,
		},
	}
	b2 := &Broker{
		Id:        "b-uid-2",
		Address:   "broker2.ns2.example.com",
		Name:      "broker2",
		Namespace: "ns2",
		DecoupleQueue: &Queue{
			Topic:        "topic2",
			Subscription: "sub2",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name3": t3,
			"name4": t4,
		},
	}
	val := &TargetsConfig{
		Brokers: map[string]*Broker{
			"ns1/broker1": b1,
			"ns2/broker2": b2,
		},
	}

	targets := &CachedTargets{}
	targets.Store(val)

	wantBytes, err := proto.Marshal(val)
	if err != nil {
		t.Fatalf("unexpected error from proto.Marshal: %v", err)
	}

	gotBytes, err := targets.Bytes()
	if err != nil {
		t.Errorf("unexpected error from targets.Byte(): %v", err)
	}

	var gotVal TargetsConfig
	if err := proto.Unmarshal(gotBytes, &gotVal); err != nil {
		t.Errorf("unexpected error from proto.Unmarshal: %v", err)
	}
	if !proto.Equal(&gotVal, val) {
		t.Errorf("got unmarshaled targets=%+v, want=%+v", gotVal, val)
	}

	// Test EqualsBytes
	if !targets.EqualsBytes(wantBytes) {
		t.Error("CachedTargets.EqualsBytes() got=false, want=true")
	}

	if targets.EqualsBytes([]byte("random")) {
		t.Error("CachedTargets.EqualBytes() with random bytes got=true, want=false")
	}
}

func TestCachedTargetsString(t *testing.T) {
	t1 := &Target{
		Id:               "uid-1",
		Name:             "name1",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "abc",
			Subscription: "abc-sub",
		},
		State: State_READY,
	}
	t2 := &Target{
		Id:               "uid-2",
		Name:             "name2",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "def",
			Subscription: "def-sub",
		},
		State: State_READY,
	}
	t3 := &Target{
		Id:               "uid-3",
		Name:             "name3",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "foo"},
		RetryQueue: &Queue{
			Topic:        "ghi",
			Subscription: "ghi-sub",
		},
		State: State_UNKNOWN,
	}
	t4 := &Target{
		Id:               "uid-4",
		Name:             "name4",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "bar"},
		RetryQueue: &Queue{
			Topic:        "jkl",
			Subscription: "jkl-sub",
		},
		State: State_UNKNOWN,
	}
	b1 := &Broker{
		Id:        "b-uid-1",
		Address:   "broker1.ns1.example.com",
		Name:      "broker1",
		Namespace: "ns1",
		DecoupleQueue: &Queue{
			Topic:        "topic1",
			Subscription: "sub1",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name1": t1,
			"name2": t2,
		},
	}
	b2 := &Broker{
		Id:        "b-uid-2",
		Address:   "broker2.ns2.example.com",
		Name:      "broker2",
		Namespace: "ns2",
		DecoupleQueue: &Queue{
			Topic:        "topic2",
			Subscription: "sub2",
		},
		State: State_READY,
		Targets: map[string]*Target{
			"name3": t3,
			"name4": t4,
		},
	}
	val := &TargetsConfig{
		Brokers: map[string]*Broker{
			"ns1/broker1": b1,
			"ns2/broker2": b2,
		},
	}

	targets := &CachedTargets{}
	targets.Store(val)

	gotStr := targets.String()
	wantStr := val.String()
	if gotStr != wantStr {
		t.Errorf("BaseTargets.String() got=%s, want=%s", gotStr, wantStr)
	}

	// Test EqualsString
	if !targets.EqualsString(wantStr) {
		t.Error("BaseTargets.EqualsString() got=false, want=true")
	}

	if targets.EqualsString("random") {
		t.Error("CachedTargets.EqualsString() with random string got=true, want=false")
	}
}
