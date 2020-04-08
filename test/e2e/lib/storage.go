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

package lib

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	"google.golang.org/api/iterator"
	v1 "k8s.io/api/core/v1"
)

func MakeStorageOrDie(client *Client,
	bucketName, storageName, targetName, pubsubServiceAccount string,
	so ...kngcptesting.CloudStorageSourceOption,
) {
	so = append(so, kngcptesting.WithCloudStorageSourceBucket(bucketName))
	so = append(so, kngcptesting.WithCloudStorageSourceSink(ServiceGVK, targetName))
	so = append(so, kngcptesting.WithCloudStorageSourceGCPServiceAccount(pubsubServiceAccount))
	eventsStorage := kngcptesting.NewCloudStorageSource(storageName, client.Namespace, so...)
	client.CreateStorageOrFail(eventsStorage)

	client.Core.WaitForResourceReadyOrFail(storageName, CloudStorageSourceTypeMeta)
}

func MakeStorageJobOrDie(client *Client, fileName, targetName string) {
	job := resources.StorageTargetJob(targetName, []v1.EnvVar{{
		Name:  "SUBJECT",
		Value: fileName,
	}, {
		Name:  "TIME",
		Value: "120",
	}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}

func AddRandomFile(ctx context.Context, t *testing.T, bucketName, fileName, project string) {
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
}

// MakeBucket retrieves the bucket name for the test. If it does not exist, it will create it.
func MakeBucket(ctx context.Context, t *testing.T, project string) string {
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

func getBucketHandle(ctx context.Context, t *testing.T, bucketName, project string) *storage.BucketHandle {
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
