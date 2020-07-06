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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudBuildSourceTestImpl tests we can create a CloudBuildSource to ready state and we can delete a CloudBuildSource and its underlying resources.
func SmokeCloudBuildSourceTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	_, deleteTopic := lib.MakeTopicWithNameIfItDoesNotExistOrDie(t, events.CloudBuildTopic)
	defer deleteTopic()

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
