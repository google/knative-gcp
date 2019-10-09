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
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/test/e2e/metrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"
	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokePubSubTestImpl tests we can create a pubsub to ready state.
func SmokePubSubTestImpl(t *testing.T) {
	topic, deleteTopic := makeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"

	client := Setup(t, true)
	defer TearDown(client)

	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace": client.Namespace,
		"topic":     topic,
		"pubsub":    psName,
	}, EndToEndConfigYaml([]string{"pubsub_test", "istio", "event_display"})...)

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
		Group:    "events.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "pubsubs",
	}); err != nil {
		t.Error(err)
	}
}

// PubSubWithTargetTestImpl tests we can receive an event from PubSub. If assertMetrics is set to true, we also assert
// for StackDriver metrics.
func PubSubWithTargetTestImpl(t *testing.T, packages map[string]string, assertMetrics bool) {
	topicName, deleteTopic := makeTopicOrDie(t)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")

	client := Setup(t, true)
	if assertMetrics {
		client.SetupStackDriverMetrics(t)
	}
	defer TearDown(client)

	config := map[string]string{
		"namespace":  client.Namespace,
		"topic":      topicName,
		"pubsub":     psName,
		"targetName": targetName,
		"targetUID":  uuid.New().String(),
	}
	for k, v := range packages {
		config[k] = v
	}

	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml([]string{"pubsub_target", "istio"})...)

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
		Group:    "events.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "pubsubs",
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
			// Log the output pods.
			if logs, err := client.LogsFor(client.Namespace, psName, gvr); err != nil {
				t.Error(err)
			} else {
				t.Logf("pubsub: %+v", logs)
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

	// Assert that we are actually sending event counts to StackDriver.
	if assertMetrics {
		sleepTime := 1 * time.Minute
		t.Logf("Sleeping %s to make sure metrics were pushed to stackdriver", sleepTime.String())
		time.Sleep(sleepTime)

		// If we reach this point, the projectID should have been set.
		projectID := os.Getenv(ProwProjectKey)
		f := map[string]interface{}{
			"metric.type":                 eventCountMetricType,
			"resource.type":               globalMetricResourceType,
			"metric.label.resource_group": pubsubResourceGroup,
			"metric.label.event_type":     v1alpha1.PubSubPublish,
			"metric.label.event_source":   v1alpha1.PubSubEventSource(projectID, topicName),
			"metric.label.namespace_name": client.Namespace,
			"metric.label.name":           psName,
			// We exit the target image before sending a response, thus check for 500.
			"metric.label.response_code":       http.StatusInternalServerError,
			"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusInternalServerError),
		}

		filter := metrics.StringifyStackDriverFilter(f)
		t.Logf("Filter expression: %s", filter)

		actualCount, err := client.StackDriverEventCountMetricFor(client.Namespace, projectID, filter)
		if err != nil {
			t.Errorf("failed to get stackdriver event count metric: %v", err)
			t.Fail()
		}
		expectedCount := int64(1)
		if *actualCount != expectedCount {
			t.Errorf("Actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
			t.Fail()
		}
	}
}
