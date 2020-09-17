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

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudAuditLogsSourceTestHelper tests if a CloudAuditLogsSource object can be created to ready state.
func SmokeCloudAuditLogsSourceTestHelper(t *testing.T, authConfig lib.AuthConfig, cloudAuditLogsSourceVersion string) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	svcName := helpers.AppendRandomString(auditlogsName + "-event-display")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)

	auditLogsConfig := lib.AuditLogsConfig{
		SinkGVK:            lib.ServiceGVK,
		SinkName:           svcName,
		AuditlogsName:      auditlogsName,
		MethodName:         lib.PubSubCreateTopicMethodName,
		Project:            project,
		ResourceName:       resourceName,
		ServiceName:        lib.PubSubServiceName,
		ServiceAccountName: authConfig.ServiceAccountName,
	}

	if cloudAuditLogsSourceVersion == "v1alpha1" {
		lib.MakeAuditLogsV1alpha1OrDie(client, auditLogsConfig)
	} else if cloudAuditLogsSourceVersion == "v1beta1" {
		lib.MakeAuditLogsV1beta1OrDie(client, auditLogsConfig)
	} else if cloudAuditLogsSourceVersion == "v1" {
		lib.MakeAuditLogsOrDie(client, auditLogsConfig)
	} else {
		t.Fatalf("SmokeCloudAuditLogsSourceTestHelper does not support CloudAuditLogsSource version: %v", cloudAuditLogsSourceVersion)
	}

}

// SmokeCloudAuditLogsSourceWithDeletionTestImpl tests if a CloudAuditLogsSource object can be created to ready state and delete a CloudAuditLogsSource resource and its underlying resources..
func SmokeCloudAuditLogsSourceWithDeletionTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	svcName := helpers.AppendRandomString(auditlogsName + "-event-display")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)

	lib.MakeAuditLogsOrDie(client, lib.AuditLogsConfig{
		SinkGVK:            lib.ServiceGVK,
		SinkName:           svcName,
		AuditlogsName:      auditlogsName,
		MethodName:         lib.PubSubCreateTopicMethodName,
		Project:            project,
		ResourceName:       resourceName,
		ServiceName:        lib.PubSubServiceName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	createdAuditLogs := client.GetAuditLogsOrFail(auditlogsName)

	topicID := createdAuditLogs.Status.TopicID
	subID := createdAuditLogs.Status.SubscriptionID
	sinkID := createdAuditLogs.Status.StackdriverSink

	createdSinkExists := lib.StackdriverSinkExists(t, sinkID)
	if !createdSinkExists {
		t.Errorf("Expected StackdriverSink%q to exist", sinkID)
	}

	createdTopicExists := lib.TopicExists(t, topicID)
	if !createdTopicExists {
		t.Errorf("Expected topic%q to exist", topicID)
	}

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}
	client.DeleteAuditLogsOrFail(auditlogsName)
	//Wait for 120 seconds for topic, subscription and notification to get deleted in gcp
	time.Sleep(resources.WaitDeletionTime)

	deletedSinkExists := lib.StackdriverSinkExists(t, sinkID)
	if deletedSinkExists {
		t.Errorf("Expected s%q StackdriverSink to get deleted", sinkID)
	}

	deletedTopicExists := lib.TopicExists(t, topicID)
	if deletedTopicExists {
		t.Errorf("Expected topic %q to get deleted", topicID)
	}

	deletedSubExists := lib.SubscriptionExists(t, subID)
	if deletedSubExists {
		t.Errorf("Expected subscription %q to get deleted", subID)
	}
}

func CloudAuditLogsSourceWithTargetTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	targetName := helpers.AppendRandomString(auditlogsName + "-target")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)

	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	// Create a target Job to receive the events.
	lib.MakeAuditLogsJobOrDie(client, lib.PubSubCreateTopicMethodName, project, resourceName, lib.PubSubServiceName, targetName, schemasv1.CloudAuditLogsLogWrittenEventType)

	// Create the CloudAuditLogsSource.
	lib.MakeAuditLogsOrDie(client, lib.AuditLogsConfig{
		SinkGVK:            lib.ServiceGVK,
		SinkName:           targetName,
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

	topicName, deleteTopic := lib.MakeTopicWithNameOrDie(t, topicName)
	defer deleteTopic()

	msg, err := client.WaitUntilJobDone(ctx, client.Namespace, targetName)
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
			if logs, err := client.LogsFor(ctx, client.Namespace, auditlogsName, lib.CloudAuditLogsSourceV1TypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("cloudauditlogssource: %+v", logs)
			}
			// Log the output of the target job pods.
			if logs, err := client.LogsFor(ctx, client.Namespace, targetName, lib.JobTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("job: %s\n", logs)
			}
			t.Fail()
		}
	}
}
