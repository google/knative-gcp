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
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudPubSubSourceTestHelper tests we can create a CloudPubSubSource to ready state.
func SmokeCloudPubSubSourceTestHelper(t *testing.T, authConfig lib.AuthConfig, cloudPubSubSourceVersion string) {
	t.Helper()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	pubSubConfig := lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: authConfig.ServiceAccountName,
	}

	if cloudPubSubSourceVersion == "v1alpha1" {
		lib.MakePubSubV1alpha1OrDie(client, pubSubConfig)
	} else if cloudPubSubSourceVersion == "v1beta1" {
		lib.MakePubSubV1beta1OrDie(client, pubSubConfig)
	} else if cloudPubSubSourceVersion == "v1" {
		lib.MakePubSubOrDie(client, pubSubConfig)
	} else {
		t.Fatalf("SmokeCloudPubSubSourceTestHelper does not support CloudPubSubSource version: %v", cloudPubSubSourceVersion)
	}
}

// SmokeCloudPubSubSourceWithDeletionTestImpl tests we can create a CloudPubSubSource to ready state and we can delete a CloudPubSubSource and its underlying resources.
func SmokeCloudPubSubSourceWithDeletionTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	// Create the PubSub source.
	lib.MakePubSubOrDie(client, lib.PubSubConfig{
		SinkGVK:            metav1.GroupVersionKind{Version: "v1", Kind: "Service"},
		PubSubName:         psName,
		SinkName:           svcName,
		TopicName:          topic,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	createdPubSub := client.GetPubSubOrFail(psName)
	subID := createdPubSub.Status.SubscriptionID

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}

	client.DeletePubSubOrFail(psName)
	//Wait for 120 seconds for subscription to get deleted in gcp
	time.Sleep(resources.WaitDeletionTime)
	deletedSubExists := lib.SubscriptionExists(t, subID)
	if deletedSubExists {
		t.Errorf("Expected subscription %q to get deleted", subID)
	}
}

// CloudPubSubSourceWithTargetTestImpl tests we can receive an event from a CloudPubSubSource.
// If assertMetrics is set to true, we also assert for StackDriver metrics.
func CloudPubSubSourceWithTargetTestImpl(t *testing.T, assertMetrics bool, authConfig lib.AuthConfig) {
	t.Helper()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")
	source := schemasv1.CloudPubSubEventSource(project, topicName)
	data := fmt.Sprintf(`{"topic":%s}`, topicName)

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	if assertMetrics {
		client.SetupStackDriverMetricsInNamespace(t)
	}
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	lib.MakePubSubTargetJobOrDie(client, source, targetName, schemasv1.CloudPubSubMessagePublishedEventType, schemasv1.CloudPubSubEventDataSchema)

	// Create the PubSub source.
	lib.MakePubSubOrDie(client, lib.PubSubConfig{
		SinkGVK:            lib.ServiceGVK,
		PubSubName:         psName,
		SinkName:           targetName,
		TopicName:          topicName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	topic := lib.GetTopic(t, topicName)

	r := topic.Publish(context.TODO(), &pubsub.Message{
		Data: []byte(data),
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
			if logs, err := client.LogsFor(client.Namespace, psName, lib.CloudPubSubSourceV1TypeMeta); err != nil {
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
		lib.AssertMetrics(t, client, topicName, psName)
	}
}
