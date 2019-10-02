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
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/test/e2e/metrics"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"

	"testing"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// makeBucket retrieves the bucket name for the test. If it does not exist, it will create it.
func makeBucket(ctx context.Context, t *testing.T, project string) string {
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
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
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	return client.Bucket(bucketName)
}

func StorageWithTestImpl(t *testing.T, packages map[string]string) {
	ctx := context.Background()
	project := os.Getenv(ProwProjectKey)

	bucketName := makeBucket(ctx, t, project)
	storageName := bucketName + "-storage"
	targetName := bucketName + "-target"

	client := Setup(t, true)
	defer TearDown(client)

	fileName := helpers.AppendRandomString("test-file-for-storage-")

	config := map[string]string{
		"namespace":   client.Namespace,
		"storage":     storageName,
		"bucket":      bucketName,
		"targetName":  targetName,
		"targetUID":   uuid.New().String(),
		"subjectName": fileName,
	}
	for k, v := range packages {
		config[k] = v
	}
	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml([]string{"storage_test", "istio"})...)

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
		Group:    "events.cloud.run",
		Version:  "v1alpha1",
		Resource: "storages",
	}

	jobGVR := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}

	if err := client.WaitForResourceReady(client.Namespace, storageName, gvr); err != nil {
		t.Error(err)
	}

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
		out := &TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output storage pods.
			if logs, err := client.LogsFor(client.Namespace, storageName, gvr); err != nil {
				t.Error(err)
			} else {
				t.Logf("storage: %+v", logs)
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

func StorageWithStackDriverMetrics(t *testing.T, packages map[string]string) {
	ctx := context.Background()
	project := os.Getenv(ProwProjectKey)

	bucketName := makeBucket(ctx, t, project)
	storageName := bucketName + "-storage"
	targetName := bucketName + "-target"

	client := Setup(t, true)
	defer TearDown(client)

	fileName := helpers.AppendRandomString("test-file-for-storage-")

	config := map[string]string{
		"namespace":   client.Namespace,
		"storage":     storageName,
		"bucket":      bucketName,
		"targetName":  targetName,
		"targetUID":   uuid.New().String(),
		"subjectName": fileName,
	}
	for k, v := range packages {
		config[k] = v
	}
	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml([]string{"storage_test", "istio"})...)

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
		Group:    "events.cloud.run",
		Version:  "v1alpha1",
		Resource: "storages",
	}

	jobGVR := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}

	if err := client.WaitForResourceReady(client.Namespace, storageName, gvr); err != nil {
		t.Error(err)
	}

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
		out := &TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output storage pods.
			if logs, err := client.LogsFor(client.Namespace, storageName, gvr); err != nil {
				t.Error(err)
			} else {
				t.Logf("storage: %+v", logs)
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

	sleepTime := 1 * time.Minute
	t.Logf("sleeping %s to make sure metrics were pushed to stackdriver", sleepTime.String())
	time.Sleep(sleepTime)

	metricClient, err := metrics.NewStackDriverMetricClient()
	if err != nil {
		t.Errorf("failed to create stackdriver metric client: %s", err.Error())
	}

	// If we reach this point, the projectID should have been set.
	projectID := os.Getenv(ProwProjectKey)
	filter := map[string]interface{}{
		"metric.type":                 eventCountMetricType,
		"resource.type":               globalMetricResourceType,
		"metric.label.resource_group": storageResourceGroup,
		"metric.label.event_type":     v1alpha1.StorageFinalize,
		"metric.label.event_source":   v1alpha1.StorageEventSource(bucketName),
		"metric.label.namespace_name": client.Namespace,
		"metric.label.name":           storageName,
		// We exit the target image before sending a response, thus check for 500.
		"metric.label.response_code":       http.StatusInternalServerError,
		"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusInternalServerError),
	}

	metricRequest := metrics.NewStackDriverListTimeSeriesRequest(projectID,
		metrics.WithStackDriverFilter(filter),
		// Starting 5 minutes back up to now.
		metrics.WithStackDriverInterval(time.Now().Add(-5*time.Minute).Unix(), time.Now().Unix()),
		// Delta counts every 2 minutes.
		metrics.WithStackDriverAlignmentPeriod(2*int64(time.Minute.Seconds())),
		metrics.WithStackDriverPerSeriesAligner(monitoringpb.Aggregation_ALIGN_DELTA),
		metrics.WithStackDriverCrossSeriesReducer(monitoringpb.Aggregation_REDUCE_COUNT),
	)

	it := metricClient.ListTimeSeries(context.TODO(), metricRequest)

	for {
		res, err := it.Next()
		if err == iterator.Done {
			t.Errorf("no metric reported")
			t.Fail()
		}
		if err != nil {
			t.Errorf("failed to iterate over result: %v", err)
			t.Fail()
		}
		t.Logf("metric: %s, resource: %s", res.Metric.Type, res.Resource.Type)
		for k, v := range res.Resource.Labels {
			t.Logf("label: %s=%s", k, v)
		}
		actualCount := res.GetPoints()[0].GetValue().GetInt64Value()
		expectedCount := int64(1)
		if actualCount == expectedCount {
			break
		}
		t.Errorf("actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
		t.Fail()
	}
}
