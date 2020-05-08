package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
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

func BrokerEventTransformationTestHelper(t *testing.T, client *lib.Client, brokerURL url.URL, brokerName string) {
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
	if done := jobDone(client, senderName, t); !done {
		t.Error("dummy event wasn't sent to broker")
		t.Failed()
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName, t); !done {
		t.Error("resp event didn't hit the target pod")
		t.Failed()
	}
}

func BrokerEventTransformationTestWithPubSubSourceHelper(t *testing.T, client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
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

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName, t); !done {
		t.Error("resp event didn't hit the target pod")
		t.Failed()
	}
}

func BrokerEventTransformationTestWithStorageSourceHelper(t *testing.T, client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	ctx := context.Background()
	project := os.Getenv(lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, t, project)
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
	lib.AddRandomFile(ctx, t, bucketName, fileName, project)

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName, t); !done {
		t.Error("resp event didn't hit the target pod")
		t.Failed()
	}
}

func BrokerEventTransformationTestWithAuditLogsSourceHelper(t *testing.T, client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	project := os.Getenv(lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	targetName := helpers.AppendRandomString(auditlogsName + "-target")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)
	// Create a target Job to receive the events.
	lib.MakeAuditLogsJobOrDie(client, lib.MethodName, project, resourceName, lib.ServiceName, targetName)
	createTriggersAndKService(client, brokerName, targetName)
	var url apis.URL = apis.URL(brokerURL)
	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Create the CloudAuditLogsSource.
	lib.MakeAuditLogsOrDie(client,
		auditlogsName,
		lib.MethodName,
		project,
		resourceName,
		lib.ServiceName,
		targetName,
		authConfig.PubsubServiceAccount,
		kngcptesting.WithCloudAuditLogsSourceSinkURI(&url),
	)

	// Audit logs source misses the topic which gets created shortly after the source becomes ready. Need to wait for a few seconds.
	// Tried with 45 seconds but the test has been quite flaky.
	time.Sleep(90 * time.Second)
	topicName, deleteTopic := lib.MakeTopicWithNameOrDie(t, topicName)
	defer deleteTopic()

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
			// Log the output cloudauditlogssource pods.
			if logs, err := client.LogsFor(client.Namespace, auditlogsName, lib.CloudAuditLogsSourceTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("cloudauditlogssource: %+v", logs)
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

func BrokerEventTransformationTestWithSchedulerSourceHelper(t *testing.T, client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
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

	msg, err := client.WaitUntilJobDone(client.Namespace, targetName)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Last termination message => %s", msg)
	if msg != "" {
		out := &lib.TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output of scheduler pods
			if logs, err := client.LogsFor(client.Namespace, sName, lib.CloudSchedulerSourceTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("scheduler log: %+v", logs)
			}

			// Log the output of the target job pods
			if logs, err := client.LogsFor(client.Namespace, targetName, lib.JobTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("addressable job: %+v", logs)
			}
			t.Fail()
		}
	}

}

func CreateKService(client *lib.Client) string {
	kserviceName := helpers.AppendRandomString("kservice")
	// Create the Knative Service.
	kservice := resources.ReceiverKService(
		kserviceName, client.Namespace)
	client.CreateUnstructuredObjOrFail(kservice)
	return kserviceName

}

func createTriggerWithKServiceSubscriber(client *lib.Client, brokerName, kserviceName string) {
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
	job := resources.TargetJob(targetName, []v1.EnvVar{{
		Name:  "TARGET",
		Value: "falldown",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))
}

func jobDone(client *lib.Client, podName string, t *testing.T) bool {
	msg, err := client.WaitUntilJobDone(client.Namespace, podName)
	if err != nil {
		t.Error(err)
		return false
	}
	if msg == "" {
		t.Error("No terminating message from the pod")
		return false
	} else {
		out := &lib.TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
			return false
		}
		if !out.Success {
			if logs, err := client.LogsFor(client.Namespace, podName, lib.JobTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("job: %s\n", logs)
			}
			return false
		}
	}
	return true
}
