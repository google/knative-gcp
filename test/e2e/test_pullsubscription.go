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

	"cloud.google.com/go/pubsub"
	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
)

// SmokePullSubscriptionTestHelper tests we can create a pull subscription to ready state.
func SmokePullSubscriptionTestHelper(t *testing.T, authConfig lib.AuthConfig, pullsubscriptionVersion string) {
	t.Helper()
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-sub"
	svcName := "event-display"

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	pullSubscriptionConfig := lib.PullSubscriptionConfig{
		SinkGVK:              lib.ServiceGVK,
		PullSubscriptionName: psName,
		SinkName:             svcName,
		TopicName:            topic,
		ServiceAccountName:   authConfig.ServiceAccountName,
	}

	if pullsubscriptionVersion == "v1alpha1" {
		lib.MakePullSubscriptionV1alpha1OrDie(client, pullSubscriptionConfig)
	} else if pullsubscriptionVersion == "v1beta1" {
		lib.MakePullSubscriptionV1beta1OrDie(client, pullSubscriptionConfig)
	} else if pullsubscriptionVersion == "v1" {
		lib.MakePullSubscriptionOrDie(client, pullSubscriptionConfig)
	} else {
		t.Fatalf("SmokePullSubscriptionTestHelper does not support PullSubscription version: %v", pullsubscriptionVersion)
	}
}

// PullSubscriptionWithTargetTestImpl tests we can receive an event from a PullSubscription.
func PullSubscriptionWithTargetTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topicName + "-sub"
	targetName := topicName + "-target"
	source := schemasv1.CloudPubSubEventSource(project, topicName)
	data := fmt.Sprintf(`{"topic":%s}`, topicName)

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	lib.MakePubSubTargetJobOrDie(client, source, targetName, schemasv1.CloudPubSubMessagePublishedEventType /*empty schema*/, "")

	// Create PullSubscription.
	lib.MakePullSubscriptionOrDie(client, lib.PullSubscriptionConfig{
		SinkGVK:              lib.ServiceGVK,
		PullSubscriptionName: psName,
		SinkName:             targetName,
		TopicName:            topicName,
		ServiceAccountName:   authConfig.ServiceAccountName,
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
			// Log the output pull subscription pods.
			if logs, err := client.LogsFor(client.Namespace, psName, lib.PullSubscriptionV1TypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("pullsubscription: %+v", logs)
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
}
