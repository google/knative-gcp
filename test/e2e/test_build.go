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

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudBuildSourceTestImpl tests we can create a CloudBuildSource to ready state and we can delete a CloudBuildSource and its underlying resources.
func SmokeCloudBuildSourceTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	lib.MakeTopicWithNameIfItDoesNotExist(t, events.CloudBuildTopic)

	buildName := helpers.AppendRandomString("build")
	svcName := "event-display"

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	// Create the Build source.
	lib.MakeBuildOrDie(client, lib.BuildConfig{
		SinkGVK:            metav1.GroupVersionKind{Version: "v1", Kind: "Service"},
		BuildName:          buildName,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	createdBuild := client.GetBuildOrFail(buildName)
	subID := createdBuild.Status.SubscriptionID

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}

	client.DeleteBuildOrFail(buildName)
	//Wait for 40 seconds for subscription to get deleted in gcp
	time.Sleep(resources.WaitDeletionTime)
	deletedSubExists := lib.SubscriptionExists(t, subID)
	if deletedSubExists {
		t.Errorf("Expected subscription %q to get deleted", subID)
	}
}

// CloudBuildSourceWithTargetTestImpl tests we can receive an event from a CloudBuildSource.
func CloudBuildSourceWithTargetTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	lib.MakeTopicWithNameIfItDoesNotExist(t, events.CloudBuildTopic)

	buildName := helpers.AppendRandomString("build")
	targetName := helpers.AppendRandomString(buildName + "-target")

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)

	defer lib.TearDown(client)

	imageName := helpers.AppendRandomString("image")
	project := os.Getenv(lib.ProwProjectKey)
	images := fmt.Sprintf("[%s]", lib.CloudBuildImage(project, imageName))

	// Create a target Job to receive the events.
	lib.MakeBuildTargetJobOrDie(client, images, targetName, schemasv1.CloudBuildSourceEventType)
	// Create the Build source.
	lib.MakeBuildOrDie(client, lib.BuildConfig{
		SinkGVK:            lib.ServiceGVK,
		BuildName:          buildName,
		SinkName:           targetName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	lib.BuildWithConfigFile(t, imageName)

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
			if logs, err := client.LogsFor(client.Namespace, buildName, lib.CloudBuildSourceTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("build: %+v", logs)
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
