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
	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

// SmokePullSubscriptionTestImpl tests we can create a pull subscription to ready state.
func SmokePullSubscriptionTestImpl(t *testing.T) {
	topic, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topic + "-sub"
	svcName := "event-display"

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	// Create PullSubscription.
	pullsubscription := kngcptesting.NewPullSubscription(psName, client.Namespace,
		kngcptesting.WithPullSubscriptionSpec(v1alpha1.PullSubscriptionSpec{
			Topic: topic,
		}),
		kngcptesting.WithPullSubscriptionSink(lib.ServiceGVK, svcName))
	client.CreatePullSubscriptionOrFail(pullsubscription)

	client.Core.WaitForResourceReadyOrFail(psName, lib.PullSubscriptionTypeMeta)
}

// PullSubscriptionWithTargetTestImpl tests we can receive an event from a PullSubscription.
func PullSubscriptionWithTargetTestImpl(t *testing.T) {
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	psName := topicName + "-sub"
	targetName := topicName + "-target"

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	job := resources.TargetJob(targetName, []v1.EnvVar{{
		Name:  "TARGET",
		Value: "falldown",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))

	// Create PullSubscription.
	pullsubscription := kngcptesting.NewPullSubscription(psName, client.Namespace,
		kngcptesting.WithPullSubscriptionSpec(v1alpha1.PullSubscriptionSpec{
			Topic: topicName,
		}), kngcptesting.WithPullSubscriptionSink(lib.ServiceGVK, targetName))
	client.CreatePullSubscriptionOrFail(pullsubscription)

	client.Core.WaitForResourceReadyOrFail(psName, lib.PullSubscriptionTypeMeta)

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
			// Log the output pull subscription pods.
			if logs, err := client.LogsFor(client.Namespace, psName, lib.PullSubscriptionTypeMeta); err != nil {
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
