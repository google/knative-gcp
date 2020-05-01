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

	"cloud.google.com/go/pubsub"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudPubSubSourceTestImpl tests we can create a CloudPubSubSource to ready state.
func SmokeCloudPubSubSourceTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-pubsub"
	svcName := "event-display"

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	// Create the PubSub source.
	lib.MakePubSubOrDie(client, metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service"}, psName, svcName, topic, authConfig.PubsubServiceAccount)

	client.Core.WaitForResourceReadyOrFail(psName, lib.CloudPubSubSourceTypeMeta)

}

// CloudPubSubSourceWithTargetTestImpl tests we can receive an event from a CloudPubSubSource.
// If assertMetrics is set to true, we also assert for StackDriver metrics.
func CloudPubSubSourceWithTargetTestImpl(t *testing.T, assertMetrics bool, authConfig lib.AuthConfig) {
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
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
	lib.MakePubSubOrDie(client, lib.ServiceGVK, psName, targetName, topicName, authConfig.PubsubServiceAccount)

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
		lib.AssertMetrics(t, client, topicName, psName)
	}
}
