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
	"os"
	"testing"
	"time"

	pkgmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func CloudStorageSourceWithTestImpl(t *testing.T, assertMetrics bool, authConfig lib.AuthConfig) {
	ctx := context.Background()
	project := os.Getenv(lib.ProwProjectKey)

	bucketName := lib.MakeBucket(ctx, t, project)
	storageName := helpers.AppendRandomString(bucketName + "-storage")
	targetName := helpers.AppendRandomString(bucketName + "-target")

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	if assertMetrics {
		client.SetupStackDriverMetrics(t)
	}
	defer lib.TearDown(client)

	fileName := helpers.AppendRandomString("test-file-for-storage")

	// Create a storage_target Job to receive the events.
	lib.MakeStorageJobOrDie(client, fileName, targetName)

	// Create the Storage source.
	lib.MakeStorageOrDie(client, bucketName, storageName, targetName, authConfig.PubsubServiceAccount)

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
