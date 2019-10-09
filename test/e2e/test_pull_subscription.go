/*
Copyright 2019 Google LLC

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

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"cloud.google.com/go/pubsub"
	"k8s.io/apimachinery/pkg/runtime/schema"
	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokePullSubscriptionTestImpl tests we can create a pull subscription to ready state.
func SmokePullSubscriptionTestImpl(t *testing.T) {
	topic, deleteTopic := makeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-sub"

	client := Setup(t, true)
	defer TearDown(client)

	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace":    client.Namespace,
		"topic":        topic,
		"subscription": psName,
	}, EndToEndConfigYaml([]string{"pull_subscription_test", "istio", "event_display"})...)

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return
	}

	// Delete deferred.
	defer func() {
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
		// Just chill for tick.
		time.Sleep(10 * time.Second)
	}()

	if err := client.WaitForResourceReady(client.Namespace, psName, schema.GroupVersionResource{
		Group:    "pubsub.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "pullsubscriptions",
	}); err != nil {
		t.Error(err)
	}
}

// PullSubscriptionWithTargetTestImpl tests we can receive an event from a PullSubscription.
func PullSubscriptionWithTargetTestImpl(t *testing.T, packages map[string]string) {
	topicName, deleteTopic := makeTopicOrDie(t)
	defer deleteTopic()

	psName := topicName + "-sub"
	targetName := topicName + "-target"

	client := Setup(t, true)
	defer TearDown(client)

	config := map[string]string{
		"namespace":    client.Namespace,
		"topic":        topicName,
		"subscription": psName,
		"targetName":   targetName,
		"targetUID":    uuid.New().String(),
	}
	for k, v := range packages {
		config[k] = v
	}

	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml([]string{"pull_subscription_target", "istio"})...)

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return
	}

	// Delete deferred.
	defer func() {
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
		// Just chill for tick.
		time.Sleep(10 * time.Second)
	}()

	gvr := schema.GroupVersionResource{
		Group:    "pubsub.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "pullsubscriptions",
	}

	jobGVR := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}

	if err := client.WaitForResourceReady(client.Namespace, psName, gvr); err != nil {
		t.Error(err)
	}

	topic := getTopic(t, topicName)

	r := topic.Publish(context.TODO(), &pubsub.Message{
		Attributes: map[string]string{
			"target": "falldown",
		},
		Data: []byte(`{"foo":bar}`),
	})
	_, err := r.Get(context.TODO())
	if err != nil {
		t.Logf("%s", err)
	}

	msg, err := client.WaitUntilJobDone(client.Namespace, targetName)
	if err != nil {
		t.Error(err)
	}
	t.Logf("Last term message => %s", msg)

	if msg != "" {
		out := &TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output pull subscription pods.
			if logs, err := client.LogsFor(client.Namespace, psName, gvr); err != nil {
				t.Error(err)
			} else {
				t.Logf("pullsubscription: %+v", logs)
			}
			// Log the output of the target job pods.
			if logs, err := client.LogsFor(client.Namespace, targetName, jobGVR); err != nil {
				t.Error(err)
			} else {
				t.Logf("job: %s\n", logs)
			}
			t.Fail()
		}
	}
}
