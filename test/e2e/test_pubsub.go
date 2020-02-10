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

	"cloud.google.com/go/pubsub"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudPubSubSourceTestImpl tests we can create a CloudPubSubSource to ready state.
func SmokeCloudPubSubSourceTestImpl(t *testing.T) {
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	// Create the PubSub source.
	eventsPubsub := kngcptesting.NewCloudPubSubSource(psName, client.Namespace,
		kngcptesting.WithCloudPubSubSourceSink(metav1.GroupVersionKind{
			Version: "v1",
			Kind:    "Service"}, svcName),
		kngcptesting.WithCloudPubSubSourceTopic(topic))
	client.CreatePubSubOrFail(eventsPubsub)

	client.Core.WaitForResourceReadyOrFail(psName, lib.CloudPubSubSourceTypeMeta)

}

// CloudPubSubSourceWithTargetTestImpl tests we can receive an event from a CloudPubSubSource.
// If assertMetrics is set to true, we also assert for StackDriver metrics.
func CloudPubSubSourceWithTargetTestImpl(t *testing.T, assertMetrics bool) {
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")

	client := lib.Setup(t, true)
	if assertMetrics {
		client.SetupStackDriverMetrics(t)
	}
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	job := resources.TargetJob(targetName, []v1.EnvVar{{
		Name:  "TARGET",
		Value: "falldown",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))

	// Create the PubSub source.
	eventsPubsub := kngcptesting.NewCloudPubSubSource(psName, client.Namespace,
		kngcptesting.WithCloudPubSubSourceSink(lib.ServiceGVK, targetName),
		kngcptesting.WithCloudPubSubSourceTopic(topicName))
	client.CreatePubSubOrFail(eventsPubsub)

	client.Core.WaitForResourceReadyOrFail(psName, lib.CloudPubSubSourceTypeMeta)

	topic := lib.GetTopic(t, topicName)

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
		out := &lib.TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output pods.
			if logs, err := client.LogsFor(client.Namespace, psName, lib.CloudPubSubSourceTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("pubsub: %+v", logs)
			}
			// Log the output of the target job pods.
			if logs, err := client.LogsFor(client.Namespace, targetName, lib.JobTypeMeta); err != nil {
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
		projectID := os.Getenv(lib.ProwProjectKey)
		f := map[string]interface{}{
			"metric.type":                 lib.EventCountMetricType,
			"resource.type":               lib.GlobalMetricResourceType,
			"metric.label.resource_group": lib.PubsubResourceGroup,
			"metric.label.event_type":     v1alpha1.CloudPubSubSourcePublish,
			"metric.label.event_source":   v1alpha1.CloudPubSubSourceEventSource(projectID, topicName),
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
