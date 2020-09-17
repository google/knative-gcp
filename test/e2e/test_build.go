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
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudBuildSourceWithDeletionTestHelper tests we can create a CloudBuildSource to ready state.
func SmokeCloudBuildSourceTestHelper(t *testing.T, authConfig lib.AuthConfig, cloudBuildSourceVersion string) {
	t.Helper()
	lib.MakeTopicWithNameIfItDoesNotExist(t, events.CloudBuildTopic)

	buildName := helpers.AppendRandomString("build")
	svcName := "event-display"

	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	buildConfig := lib.BuildConfig{
		SinkGVK:            lib.ServiceGVK,
		BuildName:          buildName,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	}

	if cloudBuildSourceVersion == "v1alpha1" {
		lib.MakeBuildV1alpha1OrDie(client, buildConfig)
	} else if cloudBuildSourceVersion == "v1beta1" {
		lib.MakeBuildV1beta1OrDie(client, buildConfig)
	} else if cloudBuildSourceVersion == "v1" {
		lib.MakeBuildOrDie(client, buildConfig)
	} else {
		t.Fatalf("SmokeCloudBuildSourceWithDeletionTestHelper does not support CloudBuildSource version: %v", cloudBuildSourceVersion)
	}
}

// SmokeCloudBuildSourceWithDeletionTestImpl tests we can create a CloudBuildSource to ready state and we can delete a CloudBuildSource and its underlying resources.
func SmokeCloudBuildSourceWithDeletionTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	lib.MakeTopicWithNameIfItDoesNotExist(t, events.CloudBuildTopic)

	buildName := helpers.AppendRandomString("build")
	svcName := "event-display"

	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

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
	//Wait for 120 seconds for subscription to get deleted in gcp
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

	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	imageName := helpers.AppendRandomString("image")
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
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
			// Log the output pods.
			if logs, err := client.LogsFor(ctx, client.Namespace, buildName, lib.CloudBuildSourceV1beta1TypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("build: %+v", logs)
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
