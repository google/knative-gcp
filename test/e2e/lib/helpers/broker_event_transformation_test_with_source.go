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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	v1 "k8s.io/api/core/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingtestlib "knative.dev/eventing/test/lib"
	eventingtestresources "knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

/*
 BrokerEventTransformationTestHelper provides the helper methods which test the following scenario:

                              5                   4
                    ------------------   --------------------
                    |                 | |                    |
          1         v	      2       | v         3          |
(Sender or Source) --->   Broker ---> trigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func BrokerEventTransformationTestHelper(client *lib.Client, brokerURL url.URL, brokerName string, needRetry bool) {
	client.T.Helper()
	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)

	// Create the Knative Service.
	kserviceName := CreateKService(client, "receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyEventType})
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyRespEventType})
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	envVar := []v1.EnvVar{{
		Name:  "BROKER_URL",
		Value: brokerURL.String()},
	}

	if needRetry {
		envVar = append(envVar, v1.EnvVar{
			Name:  "RETRY",
			Value: "true",
		})
	}

	// Create a sender Job to send the event with retry.
	senderJob := resources.SenderJob(senderName, envVar)
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

func BrokerEventTransformationMetricsTestHelper(client *lib.Client, projectID string, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	start := time.Now()

	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)

	// Create the Knative Service.
	kserviceName := createFirstNErrsReceiver(client, 2)

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyEventType})
	trigger := createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyRespEventType})
	respTrigger := createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create a sender Job to send the event with retry.
	senderJob := resources.SenderJob(senderName, []v1.EnvVar{{
		Name:  "BROKER_URL",
		Value: brokerURL.String(),
	}, {
		Name:  "RETRY",
		Value: "true",
	}})
	client.CreateJobOrFail(senderJob)

	// Check if dummy CloudEvent is sent out.
	if done := jobDone(client, senderName); !done {
		client.T.Fatal("dummy event wasn't sent to broker")
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Fatal("resp event didn't hit the target pod")
	}
	metrics.CheckAssertions(client.T,
		lib.BrokerMetricAssertion{
			ProjectID:       projectID,
			BrokerName:      brokerName,
			BrokerNamespace: client.Namespace,
			StartTime:       start,
			CountPerType: map[string]int64{
				lib.E2EDummyEventType:     1,
				lib.E2EDummyRespEventType: 1,
			},
		},
		lib.TriggerMetricAssertion{
			ProjectID:       projectID,
			BrokerName:      brokerName,
			BrokerNamespace: client.Namespace,
			StartTime:       start,
			CountPerTriggerWithRespCode: map[lib.TriggerAssertionKey]int64{
				{Name: trigger.Name, RespCode: http.StatusAccepted}: 1,
				{Name: respTrigger.Name, RespCode: http.StatusOK}:   1,
				// Metric from first two delivery attempts (which would fail).
				{Name: trigger.Name, RespCode: http.StatusBadRequest}: 2,
			},
			CountPerTriggerNoRespCode: map[lib.TriggerAssertionKey]int64{
				// For metrics without response code, we expect 3 trigger deliveries (first 2 from delivery failures).
				{Name: trigger.Name}:     3,
				{Name: respTrigger.Name}: 1,
			},
		},
	)
}

func BrokerEventTransformationTracingTestHelper(client *lib.Client, projectID string, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)

	// Create the Knative Service.
	kserviceName := CreateKService(client, "receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyEventType})
	trigger := createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter, eventingv1beta1.TriggerAnyFilter,
		map[string]interface{}{"type": lib.E2EDummyRespEventType})
	respTrigger := createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create a sender Job to send the event with retry.
	senderJob := resources.SenderJob(senderName, []v1.EnvVar{{
		Name:  "BROKER_URL",
		Value: brokerURL.String(),
	}, {
		Name:  "RETRY",
		Value: "true",
	}})
	client.CreateJobOrFail(senderJob)

	// Check if dummy CloudEvent is sent out.
	senderOutput := new(lib.SenderOutput)
	if err := jobOutput(client, senderName, senderOutput); err != nil {
		client.T.Errorf("dummy event wasn't sent to broker: %v", err)
		client.T.Failed()
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
	testTree := BrokerTestTree(client.Namespace, brokerName, trigger.Name, respTrigger.Name)
	VerifyTrace(client.T, testTree, projectID, senderOutput.TraceID)
}

func BrokerEventTransformationTestWithPubSubSourceHelper(client *lib.Client, authConfig lib.AuthConfig, brokerURL url.URL, brokerName string) {
	client.T.Helper()
	project := os.Getenv(lib.ProwProjectKey)
	topicName, deleteTopic := lib.MakeTopicOrDie(client.T)
	defer deleteTopic()

	psName := helpers.AppendRandomString(topicName + "-pubsub")
	targetName := helpers.AppendRandomString(topicName + "-target")
	data := fmt.Sprintf(`{"topic":%s}`, topicName)
	source := schemasv1.CloudPubSubEventSource(project, topicName)

	// Create a target PubSub Job to receive the events.
	lib.MakePubSubTargetJobOrDie(client, source, targetName, lib.E2EPubSubRespEventType /*empty schema*/, "")
	// Create the Knative Service.
	kserviceName := CreateKService(client, "pubsub_receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		schemasv1.CloudPubSubMessagePublishedEventType,
		map[string]interface{}{})
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		lib.E2EPubSubRespEventType,
		map[string]interface{}{})
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create the PubSub source.
	lib.MakePubSubOrDie(client, lib.PubSubConfig{
		SinkGVK:            lib.BrokerGVK,
		PubSubName:         psName,
		SinkName:           brokerName,
		TopicName:          topicName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	topic := lib.GetTopic(client.T, topicName)

	r := topic.Publish(context.TODO(), &pubsub.Message{
		Data: []byte(data),
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
	defer lib.DeleteBucket(ctx, client.T, bucketName)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	targetName := helpers.AppendRandomString(bucketName + "-target")
	source := schemasv1.CloudStorageEventSource(bucketName)
	fileName := helpers.AppendRandomString("test-file-for-storage")
	subject := schemasv1.CloudStorageEventSubject(fileName)
	// Create a target StorageJob to receive the events.
	lib.MakeStorageJobOrDie(client, source, subject, targetName, lib.E2EStorageRespEventType)
	// Create the Knative Service.
	kserviceName := CreateKService(client, "storage_receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		schemasv1.CloudStorageObjectFinalizedEventType,
		map[string]interface{}{})
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		lib.E2EStorageRespEventType,
		map[string]interface{}{})
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create the Storage source.
	lib.MakeStorageOrDie(client, lib.StorageConfig{
		SinkGVK:            lib.BrokerGVK,
		BucketName:         bucketName,
		StorageName:        storageName,
		SinkName:           brokerName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	// Add a random name file in the bucket
	lib.AddRandomFile(ctx, client.T, bucketName, fileName, project)

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
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
	lib.MakeAuditLogsJobOrDie(client, lib.PubSubCreateTopicMethodName, project, resourceName, lib.PubSubServiceName, targetName, lib.E2EAuditLogsRespType)
	// Create the Knative Service.
	kserviceName := CreateKService(client, "auditlogs_receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		schemasv1.CloudAuditLogsLogWrittenEventType,
		map[string]interface{}{})
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		lib.E2EAuditLogsRespType,
		map[string]interface{}{})
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)
	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create the CloudAuditLogsSource.
	lib.MakeAuditLogsOrDie(client, lib.AuditLogsConfig{
		SinkGVK:            lib.BrokerGVK,
		SinkName:           brokerName,
		AuditlogsName:      auditlogsName,
		MethodName:         lib.PubSubCreateTopicMethodName,
		Project:            project,
		ResourceName:       resourceName,
		ServiceName:        lib.PubSubServiceName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	client.Core.WaitForResourceReadyOrFail(auditlogsName, lib.CloudAuditLogsSourceV1TypeMeta)

	// Audit logs source misses the topic which gets created shortly after the source becomes ready. Need to wait for a few seconds.
	time.Sleep(resources.WaitCALTime)

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
	data := helpers.AppendRandomString("scheduler-source-with-broker")
	schedulerName := helpers.AppendRandomString("scheduler-e2e-test")
	targetName := helpers.AppendRandomString(schedulerName + "-target")

	lib.MakeSchedulerJobOrDie(client, data, targetName, lib.E2ESchedulerRespType)
	// Create the Knative Service.
	kserviceName := CreateKService(client, "scheduler_receiver")

	// Create a Trigger with the Knative Service subscriber.
	triggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		schemasv1.CloudSchedulerJobExecutedEventType,
		map[string]interface{}{})
	createTriggerWithKServiceSubscriber(client, brokerName, kserviceName, triggerFilter)

	// Create a Trigger with the target Service subscriber.
	respTriggerFilter := eventingtestresources.WithAttributesTriggerFilterV1Beta1(
		eventingv1beta1.TriggerAnyFilter,
		lib.E2ESchedulerRespType,
		map[string]interface{}{})
	createTriggerWithTargetServiceSubscriber(client, brokerName, targetName, respTriggerFilter)

	// Wait for ksvc, trigger ready.
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)
	// Just to make sure all resources are ready.

	// Just to make sure all resources are ready.
	time.Sleep(resources.WaitBrokercellTime)

	// Create the CloudSchedulerSource.
	lib.MakeSchedulerOrDie(client, lib.SchedulerConfig{
		SinkGVK:            lib.BrokerGVK,
		SchedulerName:      schedulerName,
		Data:               data,
		SinkName:           brokerName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName); !done {
		client.T.Error("resp event didn't hit the target pod")
		client.T.Failed()
	}
}

func CreateKService(client *lib.Client, imageName string) string {
	client.T.Helper()
	kserviceName := helpers.AppendRandomString("kservice")
	// Create the Knative Service.
	kservice := resources.ReceiverKService(
		kserviceName, client.Namespace, imageName)
	client.CreateUnstructuredObjOrFail(kservice)
	return kserviceName

}

func createFirstNErrsReceiver(client *lib.Client, firstNErrs int) string {
	client.T.Helper()
	kserviceName := helpers.AppendRandomString("kservice")
	// Create the Knative Service.
	kservice := resources.FirstNErrsReceiverKService(
		kserviceName, client.Namespace, "receiver", firstNErrs)
	client.CreateUnstructuredObjOrFail(kservice)
	return kserviceName
}

func createTriggerWithKServiceSubscriber(client *lib.Client,
	brokerName, kserviceName string,
	triggerFilter eventingtestresources.TriggerOptionV1Beta1) *eventingv1beta1.Trigger {
	client.T.Helper()
	// Please refer to the graph in the file to check what dummy trigger is used for.
	triggerName := "trigger-broker-" + brokerName
	return client.Core.CreateTriggerOrFailV1Beta1(
		triggerName,
		eventingtestresources.WithBrokerV1Beta1(brokerName),
		triggerFilter,
		eventingtestresources.WithSubscriberServiceRefForTriggerV1Beta1(kserviceName),
	)
}

func createTriggerWithTargetServiceSubscriber(client *lib.Client,
	brokerName, targetName string,
	triggerFilter eventingtestresources.TriggerOptionV1Beta1) *eventingv1beta1.Trigger {
	client.T.Helper()
	respTriggerName := "resp-broker-" + brokerName
	return client.Core.CreateTriggerOrFailV1Beta1(
		respTriggerName,
		eventingtestresources.WithBrokerV1Beta1(brokerName),
		triggerFilter,
		eventingtestresources.WithSubscriberServiceRefForTriggerV1Beta1(targetName),
	)
}

func makeTargetJobOrDie(client *lib.Client, targetName string) {
	client.T.Helper()
	job := resources.TargetJob(targetName, []v1.EnvVar{{
		// TIME (used in knockdown.Config) is the timeout for the target to receive event.
		Name:  "TIME",
		Value: "5m",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))
}

func jobDone(client *lib.Client, podName string) bool {
	client.T.Helper()
	out := &lib.TargetOutput{}
	if err := jobOutput(client, podName, out); err != nil {
		client.T.Error(err)
		return false
	}
	return true
}

func jobOutput(client *lib.Client, podName string, out lib.Output) error {
	client.T.Helper()
	msg, err := client.WaitUntilJobDone(client.Namespace, podName)
	if err != nil {
		return err
	}
	if msg == "" {
		return errors.New("no terminating message from the pod")
	}

	if err := json.Unmarshal([]byte(msg), out); err != nil {
		return err
	}
	if !out.Successful() {
		if logs, err := client.LogsFor(client.Namespace, podName, lib.JobTypeMeta); err != nil {
			return err
		} else {
			return fmt.Errorf("job: %s\n", logs)
		}
	}
	return nil
}
