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
	"net/http"
	"testing"
	"time"

	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeCloudStorageSourceTestHelper tests we can create a CloudStorageSource to ready state.
func SmokeCloudStorageSourceTestHelper(t *testing.T, authConfig lib.AuthConfig, cloudStorageSourceVersion string) {
	t.Helper()
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	ctx := context.Background()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, t, project)
	defer lib.DeleteBucket(ctx, t, bucketName)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	svcName := helpers.AppendRandomString(bucketName + "-event-display")

	storageConfig := lib.StorageConfig{
		SinkGVK:            lib.ServiceGVK,
		BucketName:         bucketName,
		StorageName:        storageName,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	}

	if cloudStorageSourceVersion == "v1alpha1" {
		lib.MakeStorageV1alpha1OrDie(client, storageConfig)
	} else if cloudStorageSourceVersion == "v1beta1" {
		lib.MakeStorageV1beta1OrDie(client, storageConfig)
	} else if cloudStorageSourceVersion == "v1" {
		lib.MakeStorageOrDie(client, storageConfig)
	} else {
		t.Fatalf("SmokeCloudStorageSourceTestHelper does not support CloudStorageSource version: %v", cloudStorageSourceVersion)
	}
}

// SmokeCloudStorageSourceWithDeletionTestImpl tests if a CloudStorageSource object can be created to ready state and delete a CloudStorageSource resource and its underlying resources..
func SmokeCloudStorageSourceWithDeletionTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	t.Helper()
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	ctx := context.Background()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, t, project)
	defer lib.DeleteBucket(ctx, t, bucketName)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	svcName := helpers.AppendRandomString(bucketName + "-event-display")

	//make the storage source
	lib.MakeStorageOrDie(client, lib.StorageConfig{
		SinkGVK:            lib.ServiceGVK,
		BucketName:         bucketName,
		StorageName:        storageName,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	createdStorage := client.GetStorageOrFail(storageName)

	topicID := createdStorage.Status.TopicID
	subID := createdStorage.Status.SubscriptionID
	notificationID := createdStorage.Status.NotificationID

	createdNotificationExists := lib.NotificationExists(t, bucketName, notificationID)
	if !createdNotificationExists {
		t.Errorf("Expected notification%q to exist", topicID)
	}

	createdTopicExists := lib.TopicExists(t, topicID)
	if !createdTopicExists {
		t.Errorf("Expected topic%q to exist", topicID)
	}

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}
	client.DeleteStorageOrFail(storageName)
	//Wait for 120 seconds for topic, subscription and notification to get deleted in gcp
	time.Sleep(resources.WaitDeletionTime)

	deletedNotificationExists := lib.NotificationExists(t, bucketName, notificationID)
	if deletedNotificationExists {
		t.Errorf("Expected notification%q to get deleted", notificationID)
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

func CloudStorageSourceWithTargetTestImpl(t *testing.T, assertMetrics bool, authConfig lib.AuthConfig) {
	t.Helper()
	ctx := context.Background()
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, t, project)
	defer lib.DeleteBucket(ctx, t, bucketName)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	targetName := helpers.AppendRandomString(bucketName + "-target")

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	if assertMetrics {
		client.SetupStackDriverMetricsInNamespace(t)
	}
	defer lib.TearDown(client)

	fileName := helpers.AppendRandomString("test-file-for-storage")
	source := schemasv1.CloudStorageEventSource(bucketName)
	subject := schemasv1.CloudStorageEventSubject(fileName)

	// Create a storage_target Job to receive the events.
	lib.MakeStorageJobOrDie(client, source, subject, targetName, schemasv1.CloudStorageObjectFinalizedEventType)

	// Create the Storage source.
	lib.MakeStorageOrDie(client, lib.StorageConfig{
		SinkGVK:            lib.ServiceGVK,
		BucketName:         bucketName,
		StorageName:        storageName,
		SinkName:           targetName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	// Add a random name file in the bucket
	lib.AddRandomFile(ctx, t, bucketName, fileName, project)

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
			// Log the output storage pods.
			if logs, err := client.LogsFor(client.Namespace, storageName, lib.CloudStorageSourceV1TypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("storage: %+v", logs)
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

	if assertMetrics {
		sleepTime := 1 * time.Minute
		t.Logf("Sleeping %s to make sure metrics were pushed to stackdriver", sleepTime.String())
		time.Sleep(sleepTime)

		f := map[string]interface{}{
			"metric.type":                 lib.EventCountMetricType,
			"resource.type":               lib.GlobalMetricResourceType,
			"metric.label.resource_group": lib.StorageResourceGroup,
			"metric.label.event_type":     schemasv1.CloudStorageObjectFinalizedEventType,
			"metric.label.event_source":   schemasv1.CloudStorageEventSource(bucketName),
			"metric.label.namespace_name": client.Namespace,
			"metric.label.name":           storageName,
			// We exit the target image before sending a response, thus check for 500.
			"metric.label.response_code":       http.StatusInternalServerError,
			"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusInternalServerError),
		}

		filter := metrics.StringifyStackDriverFilter(f)
		t.Logf("Filter expression: %s", filter)

		actualCount, err := client.StackDriverEventCountMetricFor(client.Namespace, project, filter)
		if err != nil {
			t.Fatalf("failed to get stackdriver event count metric: %v", err)
		}
		expectedCount := int64(1)
		if actualCount != expectedCount {
			t.Errorf("Actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
		}
	}
}
