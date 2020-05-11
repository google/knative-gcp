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

package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	v1 "k8s.io/api/core/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingtestlib "knative.dev/eventing/test/lib"
	eventingtestresources "knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

/*
 BrokerEventTransformationTestHelper provides the helper methods which test the following scenario:

                              5                   4
                    ------------------   --------------------
                    |                 | |                    |
          1         v	      2       | v         3          |
(Sender) --->   Broker ---> dummyTrigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func BrokerEventTransformationTestHelper(client *lib.Client, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)

	createTriggersAndKService(client, brokerName, targetName)

	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create a sender Job to sender the event.
	senderJob := resources.SenderJob(senderName, []v1.EnvVar{{
		Name:  "BROKER_URL",
		Value: brokerURL.String(),
	}})
	client.CreateJobOrFail(senderJob)

	// Check if dummy CloudEvent is sent out.
	if done := jobDone(client, senderName); !done {
		client.T.Error("dummy event wasn't sent to broker")
		client.T.Failed()
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func BrokerEventTransformationTestWithPubSubSourceHelper(client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	topicName, deleteTopic := lib.MakeTopicOrDie(client.T)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)
	createTriggersAndKService(client, brokerName, targetName)
	var url apis.URL = apis.URL(brokerURL)
	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create the PubSub source.
	lib.MakePubSubOrDie(client,
		lib.ServiceGVK,
		psName,
		targetName,
		topicName,
		authConfig.PubsubServiceAccount,
		kngcptesting.WithCloudPubSubSourceSinkURI(&url),
	)

	topic := lib.GetTopic(client.T, topicName)

	r := topic.Publish(context.TODO(), &pubsub.Message{
		Attributes: map[string]string{
			"target": "falldown",
		},
		Data: []byte(`{"foo":bar}`),
	})

	_, err := r.Get(context.TODO())
	if err != nil {
		client.T.Logf("%s", err)
	}

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func BrokerEventTransformationTestWithStorageSourceHelper(client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	ctx := context.Background()
	project := os.Getenv(lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, client.T, project)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	targetName := helpers.AppendRandomString(bucketName + "-target")
	fileName := helpers.AppendRandomString("test-file-for-storage")
	// Create a target StorageJob to receive the events.
	lib.MakeStorageJobOrDie(client, fileName, targetName)
	createTriggersAndKService(client, brokerName, targetName)
	var url apis.URL = apis.URL(brokerURL)
	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create the Storage source.
	lib.MakeStorageOrDie(
		client,
		bucketName,
		storageName,
		targetName,
		authConfig.PubsubServiceAccount,
		kngcptesting.WithCloudStorageSourceSinkURI(&url),
	)

	// Add a random name file in the bucket
	lib.AddRandomFile(ctx, client.T, bucketName, fileName, project)

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func BrokerEventTransformationTestWithAuditLogsSourceHelper(client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	project := os.Getenv(lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	targetName := helpers.AppendRandomString(auditlogsName + "-target")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)
	// Create a target Job to receive the events.
	lib.MakeAuditLogsJobOrDie(client, lib.PubSubCreateTopicMethodName, project, resourceName, lib.PubSubServiceName, targetName)
	createTriggersAndKService(client, brokerName, targetName)
	var url apis.URL = apis.URL(brokerURL)
	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create the CloudAuditLogsSource.
	lib.MakeAuditLogsOrDie(client,
		auditlogsName,
		lib.PubSubCreateTopicMethodName,
		project,
		resourceName,
		lib.PubSubServiceName,
		targetName,
		authConfig.PubsubServiceAccount,
		kngcptesting.WithCloudAuditLogsSourceSinkURI(&url),
	)

	client.Core.WaitForResourceReadyOrFail(auditlogsName, lib.CloudAuditLogsSourceTypeMeta)

	// Audit logs source misses the topic which gets created shortly after the source becomes ready. Need to wait for a few seconds.
	// Tried with 45 seconds but the test has been quite flaky.
	time.Sleep(90 * time.Second)
	topicName, deleteTopic := lib.MakeTopicWithNameOrDie(client.T, topicName)
	defer deleteTopic()

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func BrokerEventTransformationTestWithSchedulerSourceHelper(client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	data := "my test data"
	targetName := "event-display"
	sName := "scheduler-test"
	// Create a target Job to receive the events.
	lib.MakeSchedulerJobOrDie(client, data, targetName)
	createTriggersAndKService(client, brokerName, targetName)

	var url apis.URL = apis.URL(brokerURL)
	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create the CloudSchedulerSource.
	lib.MakeSchedulerOrDie(client, sName, data, targetName, authConfig.PubsubServiceAccount,
		kngcptesting.WithCloudSchedulerSourceSinkURI(&url),
	)

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func CreateKService(client *lib.Client) string {
	client.T.Helper()
	kserviceName := helpers.AppendRandomString("kservice")
	// Create the Knative Service.
	kservice := resources.ReceiverKService(
		kserviceName, client.Namespace)
	client.CreateUnstructuredObjOrFail(kservice)
	return kserviceName

}

func createTriggerWithKServiceSubscriber(client *lib.Client, brokerName, kserviceName string) {
	client.T.Helper()
	// Please refer to the graph in the file to check what dummy trigger is used for.
	dummyTriggerName := "dummy-broker-" + brokerName
	client.Core.CreateTriggerOrFail(
		dummyTriggerName,
		eventingtestresources.WithBroker(brokerName),
		eventingtestresources.WithAttributesTriggerFilter(
			eventingv1alpha1.TriggerAnyFilter, eventingv1alpha1.TriggerAnyFilter,
			map[string]interface{}{"type": "e2e-testing-dummy"}),
		eventingtestresources.WithSubscriberServiceRefForTrigger(kserviceName),
	)
}

func createTriggerWithTargetServiceSubscriber(client *lib.Client, brokerName, targetName string) {
	client.T.Helper()
	respTriggerName := "resp-broker-" + brokerName
	client.Core.CreateTriggerOrFail(
		respTriggerName,
		eventingtestresources.WithBroker(brokerName),
		eventingtestresources.WithAttributesTriggerFilter(
			eventingv1alpha1.TriggerAnyFilter, eventingv1alpha1.TriggerAnyFilter,
			map[string]interface{}{"type": "e2e-testing-resp"}),
		eventingtestresources.WithSubscriberServiceRefForTrigger(targetName),
	)
}

func createTriggersAndKService(client *lib.Client, brokerName, targetName string) {
	client.T.Helper()
	// Create the Knative Service.
	kserviceName := CreateKService(client)

	// Create a Trigger with the Knative Service subscriber.
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName)

	// Create a Trigger with the target Service subscriber.
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)
}

func makeTargetJobOrDie(client *lib.Client, targetName string) {
	client.T.Helper()
	job := resources.TargetJob(targetName, []v1.EnvVar{{
		Name:  "TARGET",
		Value: "falldown",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))
}

func jobDone(client *lib.Client, podName string) bool {
	client.T.Helper()
	msg, err := client.WaitUntilJobDone(client.Namespace, podName)
	if err != nil {
		client.T.Error(err)
		return false
	}
	if msg == "" {
		client.T.Error("No terminating message from the pod")
		return false
	}

	out := &lib.TargetOutput{}
	if err := json.Unmarshal([]byte(msg), out); err != nil {
		client.T.Error(err)
		return false
	}
	if !out.Success {
		if logs, err := client.LogsFor(client.Namespace, podName, lib.JobTypeMeta); err != nil {
			client.T.Error(err)
		} else {
			client.T.Logf("job: %s\n", logs)
		}
		return false
	}
	return true
}
