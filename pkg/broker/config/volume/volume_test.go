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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/knative-gcp/pkg/broker/config"
)

func TestSyncConfigFromFile(t *testing.T) {
	data := &config.TargetsConfig{
		Namespaces: map[string]*config.NamespacedTargets{
			"ns1": &config.NamespacedTargets{
				Names: map[string]*config.Target{
					"name1": {
						Id:                "uid-1",
						Name:              "name1",
						Namespace:         "ns1",
						FilterAttributes:  map[string]string{"app": "foo"},
						RetryTopic:        "abc",
						RetrySubscription: "abc-sub",
						State:             config.Target_READY,
					},
					"name2": {
						Id:                "uid-2",
						Name:              "name2",
						Namespace:         "ns1",
						FilterAttributes:  map[string]string{"app": "bar"},
						RetryTopic:        "def",
						RetrySubscription: "def-sub",
						State:             config.Target_READY,
					},
				},
			},
			"ns2": &config.NamespacedTargets{
				Names: map[string]*config.Target{
					"name3": {
						Id:                "uid-3",
						Name:              "name3",
						Namespace:         "ns2",
						FilterAttributes:  map[string]string{"app": "bar"},
						RetryTopic:        "ghi",
						RetrySubscription: "ghi-sub",
						State:             config.Target_UNKNOWN,
					},
					"name4": {
						Id:                "uid-4",
						Name:              "name4",
						Namespace:         "ns2",
						FilterAttributes:  map[string]string{"app": "foo"},
						RetryTopic:        "jkl",
						RetrySubscription: "jkl-sub",
						State:             config.Target_UNKNOWN,
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

	targets, err := NewTargetsFromFile(WithPath(tmp.Name()))
	if err != nil {
		t.Fatalf("unexpected error from NewTargetsFromFile: %v", err)
	}

	gotTargets := targets.(*Targets).Internal.Load().(*config.TargetsConfig)
	if !proto.Equal(data, gotTargets) {
		t.Errorf("initial targets got=%+v, want=%+v", gotTargets, data)
	}

	data.GetNamespaces()["ns1"].GetNames()["name1"] = &config.Target{
		Id:                "uid-1",
		Name:              "name1",
		Namespace:         "ns1",
		FilterAttributes:  map[string]string{"app": "zzz"},
		RetryTopic:        "abc",
		RetrySubscription: "abc-sub",
		State:             config.Target_UNKNOWN,
	}
	data.GetNamespaces()["ns2"].GetNames()["name3"] = &config.Target{
		Id:                "uid-3",
		Name:              "name3",
		Namespace:         "ns2",
		FilterAttributes:  map[string]string{"app": "xxx"},
		RetryTopic:        "ghi",
		RetrySubscription: "ghi-sub",
		State:             config.Target_READY,
	}
	delete(data.GetNamespaces()["ns2"].GetNames(), "name4")
	b, _ = proto.Marshal(data)
	ioutil.WriteFile(tmp.Name(), b, 0644)

	<-time.After(3 * time.Second)

	gotTargets = targets.(*Targets).Internal.Load().(*config.TargetsConfig)
	if !proto.Equal(data, gotTargets) {
		t.Errorf("updated targets got=%+v, want%+v", gotTargets, data)
	}
}
