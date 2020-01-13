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

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	serviceName = "pubsub.googleapis.com"
	methodName  = " google.pubsub.v1.Publisher.CreateTopic"
)

func AuditLogsSourceWithTestImpl(t *testing.T, packages map[string]string) {
	project := os.Getenv(ProwProjectKey)

	auditlogsName := helpers.AppendRandomString("auditlogs-e2e-test")
	targetName := helpers.AppendRandomString(auditlogsName + "-target")
	topicName := helpers.AppendRandomString(auditlogsName + "-topic")
	resourceName := fmt.Sprintf("projects/%s/topics/%s", project, topicName)

	client := Setup(t, true)
	defer TearDown(client)

	config := map[string]string{
		"namespace":       client.Namespace,
		"auditlogssource": auditlogsName,
		"project":         project,
		"serviceName":     serviceName,
		"methodName":      methodName,
		"resourceName":    resourceName,
		"targetName":      targetName,
		"targetUID":       uuid.New().String(),
		"type":            v1alpha1.AuditLogEventType,
		"source":          fmt.Sprintf("%s/projects/%s", serviceName, project),
		"subject":         fmt.Sprintf("%s/%s", serviceName, resourceName),
	}
	for k, v := range packages {
		config[k] = v
	}
	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml([]string{"auditlogs_test", "istio"})...)

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return
	}

	//Delete deferred.
	defer func() {
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to delete, %s", err)
		}
		// Just chill for tick.
		time.Sleep(60 * time.Second)
	}()

	gvr := schema.GroupVersionResource{
		Group:    "events.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "auditlogssources",
	}

	jobGVR := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}

	if err := client.WaitForResourceReady(client.Namespace, auditlogsName, gvr); err != nil {
		t.Error(err)
	}

	//audit logs source misses the topic which gets created shortly after the source becomes ready. Need to wait for a few seconds
	time.Sleep(45 * time.Second)
	topicName, deleteTopic := makeTopicOrDieWithTopicName(t, topicName)
	defer deleteTopic()

	msg, err := client.WaitUntilJobDone(client.Namespace, targetName)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Last term message => %s", msg)

	if msg != "" {
		out := &TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output auditlogssource pods.
			if logs, err := client.LogsFor(client.Namespace, auditlogsName, gvr); err != nil {
				t.Error(err)
			} else {
				t.Logf("auditlogssource: %+v", logs)
			}
			// Log the output of the target job pods.
			if logs, err := client.LogsFor(client.Namespace, targetName, jobGVR); err != nil {
				t.Error(err)
			} else {
				t.Logf("job: %s\n", logs)
			}
			t.Fail()
		}
	}
}
