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
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/test/helpers"
	"os"
	"strings"
	"time"

	"testing"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// makeBucket will get an existing bucket or create a new one if it is not existed
func makeBucket(t *testing.T) string {
	ctx := context.Background()
	project := os.Getenv(ProwProjectKey)
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create storage client, %s", err.Error())
	}
	it := client.Buckets(ctx, project)
	bucketName := "storage-e2e-test-knative-gcp"
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
		if strings.Compare(bucketAttrs.Name, bucketName) == 0 {
			break
		}
	}
	return bucketName
}

func getBucketHandle(t *testing.T, bucketName string) *storage.BucketHandle {
	ctx := context.Background()
	// Prow sticks the project in this key
	project := os.Getenv(ProwProjectKey)
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

	bucketName := makeBucket(t)
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
	bucketHandle := getBucketHandle(t, bucketName)
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
	// Uncomment the following line to get log from target pod.
	//logs, err := client.LogsFor(client.Namespace, targetName, jobGVR)
	//t.Logf(logs)
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
