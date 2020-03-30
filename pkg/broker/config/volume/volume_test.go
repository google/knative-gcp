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

package volume

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

func TestSyncConfigFromFile(t *testing.T) {
	data := &config.TargetsConfig{
		Brokers: map[string]*config.Broker{
			"ns1/broker1": {
				Id:        "b-uid-1",
				Address:   "broker1.ns1.example.com",
				Name:      "broker1",
				Namespace: "ns1",
				DecoupleQueue: &config.Queue{
					Topic:        "topic1",
					Subscription: "sub1",
				},
				State: config.State_READY,
				Targets: map[string]*config.Target{
					"name1": {
						Id:               "uid-1",
						Name:             "name1",
						Namespace:        "ns1",
						FilterAttributes: map[string]string{"app": "foo"},
						RetryQueue: &config.Queue{
							Topic:        "abc",
							Subscription: "abc-sub",
						},
						State: config.State_READY,
					},
					"name2": {
						Id:               "uid-2",
						Name:             "name2",
						Namespace:        "ns1",
						FilterAttributes: map[string]string{"app": "bar"},
						RetryQueue: &config.Queue{
							Topic:        "def",
							Subscription: "def-sub",
						},
						State: config.State_READY,
					},
				},
			},
			"ns2/broker2": {
				Id:        "b-uid-2",
				Address:   "broker2.ns2.example.com",
				Name:      "broker2",
				Namespace: "ns2",
				DecoupleQueue: &config.Queue{
					Topic:        "topic2",
					Subscription: "sub2",
				},
				State: config.State_READY,
				Targets: map[string]*config.Target{
					"name3": {
						Id:               "uid-3",
						Name:             "name3",
						Namespace:        "ns2",
						FilterAttributes: map[string]string{"app": "foo"},
						RetryQueue: &config.Queue{
							Topic:        "ghi",
							Subscription: "ghi-sub",
						},
						State: config.State_UNKNOWN,
					},
					"name4": {
						Id:               "uid-4",
						Name:             "name4",
						Namespace:        "ns2",
						FilterAttributes: map[string]string{"app": "bar"},
						RetryQueue: &config.Queue{
							Topic:        "jkl",
							Subscription: "jkl-sub",
						},
						State: config.State_UNKNOWN,
					},
				},
			},
		},
	}

	b, _ := proto.Marshal(data)
	dir, err := ioutil.TempDir("", "configtest-*")
	if err != nil {
		t.Fatalf("unexpected error from creating temp dir: %v", err)
	}
	tmp, err := ioutil.TempFile(dir, "test-*")
	if err != nil {
		t.Fatalf("unexpected error from creating temp file: %v", err)
	}
	defer func() {
		tmp.Close()
		os.RemoveAll(dir)
	}()
	tmp.Write(b)
	tmp.Close()

	ch := make(chan struct{}, 1)

	targets, err := NewTargetsFromFile(WithPath(tmp.Name()), WithNotifyChan(ch))
	if err != nil {
		t.Fatalf("unexpected error from NewTargetsFromFile: %v", err)
	}

	gotTargets := targets.(*Targets).Load()
	if !proto.Equal(data, gotTargets) {
		t.Errorf("initial targets got=%+v, want=%+v", gotTargets, data)
	}

	data.Brokers["ns1/broker1"].Targets["name1"] = &config.Target{
		Id:               "uid-1",
		Name:             "name1",
		Namespace:        "ns1",
		FilterAttributes: map[string]string{"app": "zzz"},
		RetryQueue: &config.Queue{
			Topic:        "abc",
			Subscription: "abc-sub",
		},
		State: config.State_UNKNOWN,
	}
	data.Brokers["ns2/broker2"].Targets["name3"] = &config.Target{
		Id:               "uid-3",
		Name:             "name3",
		Namespace:        "ns2",
		FilterAttributes: map[string]string{"app": "xxx"},
		RetryQueue: &config.Queue{
			Topic:        "ghi",
			Subscription: "ghi-sub",
		},
		State: config.State_READY,
	}

	delete(data.Brokers["ns2/broker2"].Targets, "name4")
	b, _ = proto.Marshal(data)
	ioutil.WriteFile(tmp.Name(), b, 0644)

	<-ch

	gotTargets = targets.(*Targets).Load()
	if !proto.Equal(data, gotTargets) {
		t.Errorf("updated targets got=%+v, want%+v", gotTargets, data)
	}
}
