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
	"cloud.google.com/go/pubsub"
	"context"
	"fmt"
	"knative.dev/pkg/test/helpers"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	ProwProjectKey = "E2E_PROJECT_ID" //"GCP_PROJECT"
)

func makeTopicOrDie(t *testing.T) string {
	ctx := context.Background()
	// Prow sticks teh project in this key
	project := os.Getenv(ProwProjectKey)
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	topicName := helpers.AppendRandomString("ps-e2e-test")
	topic := client.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		t.Fatalf("failed to verify topic exists, %s", err)
	} else if exists {
		t.Fatalf("topic already exists: %q", topicName)
	} else {
		topic, err = client.CreateTopic(ctx, topicName)
		if err != nil {
			t.Fatalf("failed to create topic, %s", err)
		}
	}
	return topicName
}

func deleteTopicOrDie(t *testing.T, topicName string) {
	ctx := context.Background()
	// Prow sticks teh project in this key
	project := os.Getenv(ProwProjectKey)
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	client, err := pubsub.NewClient(ctx, project)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	topic := client.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		t.Fatalf("failed to verify topic exists, %s", err)
	} else if exists {
		if err := topic.Delete(ctx); err != nil {
			t.Fatalf("failed to delete topic, %s", err)
		}
	}
}

// SmokePullSubscriptionTestImpl tests we can create a pull subscription to ready state.
func SmokePullSubscriptionTestImpl(t *testing.T) {
	topic := makeTopicOrDie(t)
	psName := topic + "-sub"

	client := Setup(t, true)
	defer TearDown(client)

	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	yamls := []string{
		fmt.Sprintf("%s/config/pull_subscription_test/", dir),
		fmt.Sprintf("%s/config/istio/", dir),
		fmt.Sprintf("%s/config/event_display/", dir),
	}
	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace":    client.Namespace,
		"topic":        topic,
		"subscription": psName,
	}, yamls...)

	// Delete deferred.
	defer func() {
		// Just chill for tick.
		time.Sleep(20 * time.Second)
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
		deleteTopicOrDie(t, topic)
	}()

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return
	}

	if err := client.WaitForResourceReady(client.Namespace, psName, schema.GroupVersionResource{
		Group:    "pubsub.cloud.run",
		Version:  "v1alpha1",
		Resource: "pullsubscriptions",
	}); err != nil {
		t.Error(err)
	}
}
