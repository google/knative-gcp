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
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	serviceName = "pubsub.googleapis.com"
	methodName  = "google.pubsub.v1.Publisher.CreateTopic"
)

func CloudAuditLogsSourceWithTestImpl(t *testing.T) {
	project := os.Getenv(lib.ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	targetName := helpers.AppendRandomString(auditlogsName + "-target")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	job := resources.AuditLogsTargetJob(targetName, []v1.EnvVar{{
		Name:  "SERVICENAME",
		Value: serviceName,
	}, {
		Name:  "METHODNAME",
		Value: methodName,
	}, {
		Name:  "RESOURCENAME",
		Value: resourceName,
	}, {
		Name:  "TYPE",
		Value: v1alpha1.CloudAuditLogsSourceEvent,
	}, {
		Name:  "SOURCE",
		Value: v1alpha1.CloudAuditLogsSourceEventSource(serviceName, fmt.Sprintf("projects/%s", project)),
	}, {
		Name:  "SUBJECT",
		Value: resourceName,
	}, {
		Name:  "TIME",
		Value: "360",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))

	// Create the CloudAuditLogsSource.
	eventsAuditLogs := kngcptesting.NewCloudAuditLogsSource(auditlogsName, client.Namespace,
		kngcptesting.WithCloudAuditLogsSourceServiceName(serviceName),
		kngcptesting.WithCloudAuditLogsSourceMethodName(methodName),
		kngcptesting.WithCloudAuditLogsSourceProject(project),
		kngcptesting.WithCloudAuditLogsSourceResourceName(resourceName),
		kngcptesting.WithCloudAuditLogsSourceSink(lib.ServiceGVK, targetName))
	client.CreateAuditLogsOrFail(eventsAuditLogs)

	client.Core.WaitForResourceReadyOrFail(auditlogsName, lib.CloudAuditLogsSourceTypeMeta)

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
