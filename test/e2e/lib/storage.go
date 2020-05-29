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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeStorageOrDie(client *Client,
	sinkGVK metav1.GroupVersionKind, bucketName, storageName, sinkName, pubsubServiceAccount string,
	so ...kngcptesting.CloudStorageSourceOption,
) {
	client.T.Helper()
	so = append(so, kngcptesting.WithCloudStorageSourceBucket(bucketName))
	so = append(so, kngcptesting.WithCloudStorageSourceSink(sinkGVK, sinkName))
	so = append(so, kngcptesting.WithCloudStorageSourceGCPServiceAccount(pubsubServiceAccount))
	eventsStorage := kngcptesting.NewCloudStorageSource(storageName, client.Namespace, so...)
	client.CreateStorageOrFail(eventsStorage)

	client.Core.WaitForResourceReadyOrFail(storageName, CloudStorageSourceTypeMeta)
}

func MakeStorageJobOrDie(client *Client, source, fileName, targetName, eventType string) {
	client.T.Helper()
	job := resources.StorageTargetJob(targetName, []v1.EnvVar{
		{
			Name:  "TYPE",
			Value: eventType,
		},
		{
			Name:  "SOURCE",
			Value: source,
		},
		{
			Name:  "SUBJECT",
			Value: fileName,
		}, {
			Name:  "TIME",
			Value: "6m",
		}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}

func AddRandomFile(ctx context.Context, t *testing.T, bucketName, fileName, project string) {
	t.Helper()
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
	t.Helper()
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
	t.Helper()
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

func NotificationExists(t *testing.T, bucketName, notificationID string) bool {
	t.Helper()
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create storage client, %s", err.Error())
	}
	defer client.Close()

	client.Bucket(bucketName)
	bucket := client.Bucket(bucketName)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch existing notifications %s", err.Error())
	}

	if _, ok := notifications[notificationID];ok {
		return true
	}
	return  false

}
