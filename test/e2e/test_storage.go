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
	"net/http"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	v1 "k8s.io/api/core/v1"
	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"github.com/google/knative-gcp/test/e2e/lib/resources"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// makeBucket retrieves the bucket name for the test. If it does not exist, it will create it.
func makeBucket(ctx context.Context, t *testing.T, project string) string {
	if project == "" {
		t.Fatalf("failed to find %q in envvars", lib.ProwProjectKey)
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create storage client, %s", err.Error())
	}
	it := client.Buckets(ctx, project)
	bucketName := "storage-e2e-test-" + project
	// Iterate buckets to check if there has a bucket for e2e test
	for {
		bucketAttrs, err := it.Next()
		if err == iterator.Done {
			// Create a new bucket if there is no existing e2e test bucket
			bucket := client.Bucket(bucketName)
			if e := bucket.Create(ctx, project, &storage.BucketAttrs{}); e != nil {
				t.Fatalf("failed to create bucket, %s", e.Error())
			}
			break
		}
		if err != nil {
			t.Fatalf("failed to list buckets, %s", err.Error())
		}
		// Break iteration if there has a bucket for e2e test
		if bucketAttrs.Name == bucketName {
			break
		}
	}
	return bucketName
}

func getBucketHandle(ctx context.Context, t *testing.T, bucketName string, project string) *storage.BucketHandle {
	// Prow sticks the project in this key
	if project == "" {
		t.Fatalf("failed to find %q in envvars", lib.ProwProjectKey)
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	return client.Bucket(bucketName)
}

func CloudStorageSourceWithTestImpl(t *testing.T, assertMetrics bool) {
	ctx := context.Background()
	project := os.Getenv(lib.ProwProjectKey)

	bucketName := makeBucket(ctx, t, project)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	targetName := helpers.AppendRandomString(bucketName + "-target")

	client := lib.Setup(t, true)
	if assertMetrics {
		client.SetupStackDriverMetrics(t)
	}
	defer lib.TearDown(client)

	fileName := helpers.AppendRandomString("test-file-for-storage")

	// Create a storage_target Job to receive the events.
	job := resources.StorageTargetJob(targetName, []v1.EnvVar{{
		Name:  "SUBJECT",
		Value: fileName,
	}, {
		Name:  "TIME",
		Value: "120",
	}})
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))

	// Create the Storage source.
	eventsStorage := kngcptesting.NewCloudStorageSource(storageName, client.Namespace,
		kngcptesting.WithCloudStorageSourceBucket(bucketName),
		kngcptesting.WithCloudStorageSourceSink(lib.ServiceGVK, targetName))
	client.CreateStorageOrFail(eventsStorage)

	client.Core.WaitForResourceReadyOrFail(storageName, lib.CloudStorageSourceTypeMeta)

	// Add a random name file in the bucket
	bucketHandle := getBucketHandle(ctx, t, bucketName, project)
	wc := bucketHandle.Object(fileName).NewWriter(ctx)
	// Write some text to object
	if _, err := fmt.Fprintf(wc, "e2e test for storage importer.\n"); err != nil {
		t.Error(err)
	}
	if err := wc.Close(); err != nil {
		t.Error(err)
	}

	// Delete test file deferred
	defer func() {
		o := bucketHandle.Object(fileName)
		if err := o.Delete(ctx); err != nil {
			t.Error(err)
		}
	}()

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
			if logs, err := client.LogsFor(client.Namespace, storageName, lib.CloudStorageSourceTypeMeta); err != nil {
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

		// If we reach this point, the projectID should have been set.
		projectID := os.Getenv(lib.ProwProjectKey)
		f := map[string]interface{}{
			"metric.type":                 lib.EventCountMetricType,
			"resource.type":               lib.GlobalMetricResourceType,
			"metric.label.resource_group": lib.StorageResourceGroup,
			"metric.label.event_type":     v1alpha1.CloudStorageSourceFinalize,
			"metric.label.event_source":   v1alpha1.CloudStorageSourceEventSource(bucketName),
			"metric.label.namespace_name": client.Namespace,
			"metric.label.name":           storageName,
			// We exit the target image before sending a response, thus check for 500.
			"metric.label.response_code":       http.StatusInternalServerError,
			"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusInternalServerError),
		}

		filter := metrics.StringifyStackDriverFilter(f)
		t.Logf("Filter expression: %s", filter)

		actualCount, err := client.StackDriverEventCountMetricFor(client.Namespace, projectID, filter)
		if err != nil {
			t.Errorf("failed to get stackdriver event count metric: %v", err)
			t.Fail()
		}
		expectedCount := int64(1)
		if *actualCount != expectedCount {
			t.Errorf("Actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
			t.Fail()
		}
	}
}
