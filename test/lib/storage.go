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
	"time"

	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
	reconcilertestingv1beta1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1beta1"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/helpers"

	"github.com/google/knative-gcp/test/lib/resources"
)

type StorageConfig struct {
	SinkGVK            metav1.GroupVersionKind
	BucketName         string
	StorageName        string
	SinkName           string
	ServiceAccountName string
	Options            []reconcilertestingv1.CloudStorageSourceOption
}

func MakeStorageOrDie(client *Client, config StorageConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1.CloudStorageSourceOption, 0)
	so = append(so, reconcilertestingv1.WithCloudStorageSourceBucket(config.BucketName))
	so = append(so, reconcilertestingv1.WithCloudStorageSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1.WithCloudStorageSourceServiceAccount(config.ServiceAccountName))
	eventsStorage := reconcilertestingv1.NewCloudStorageSource(config.StorageName, client.Namespace, so...)
	client.CreateStorageOrFail(eventsStorage)
	// Storage source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.StorageName, CloudStorageSourceV1TypeMeta)
}

func MakeStorageV1beta1OrDie(client *Client, config StorageConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1beta1.CloudStorageSourceOption, 0)
	so = append(so, reconcilertestingv1beta1.WithCloudStorageSourceBucket(config.BucketName))
	so = append(so, reconcilertestingv1beta1.WithCloudStorageSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1beta1.WithCloudStorageSourceServiceAccount(config.ServiceAccountName))
	eventsStorage := reconcilertestingv1beta1.NewCloudStorageSource(config.StorageName, client.Namespace, so...)
	client.CreateStorageV1beta1OrFail(eventsStorage)
	// Storage source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.StorageName, CloudStorageSourceV1beta1TypeMeta)
}

func MakeStorageJobOrDie(client *Client, source, subject, targetName, eventType string) {
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
			Value: subject,
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
	opt := option.WithQuotaProject(project)
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		t.Fatalf("failed to create storage client, %s", err.Error())
	}
	it := client.Buckets(ctx, project)
	// Name should be between 3-63 characters. https://cloud.google.com/storage/docs/naming-buckets
	bucketName := helpers.AppendRandomString("storage-e2e-test")
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

func DeleteBucket(ctx context.Context, t *testing.T, bucketName string) {
	t.Helper()
	project := GetEnvOrFail(t, ProwProjectKey)
	opt := option.WithQuotaProject(project)
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		t.Fatalf("failed to create storage client, %s", err.Error())
	}
	// Load the Bucket.
	bucket := client.Bucket(bucketName)

	// Check whether bucket exists or not
	if _, err := bucket.Attrs(ctx); err != nil {
		// If the bucket was already deleted, we are good
		if err == storage.ErrBucketNotExist {
			t.Logf("Bucket %s already deleted", bucketName)
			return
		} else {
			t.Errorf("Failed to fetch attrs of Bucket %s", bucketName)
		}
	}

	// If we fail to delete the bucket, we will fail the test so that users are aware that
	// we leaked a bucket in the project. If this makes the test flake, then we can revisit and maybe avoid failing.
	if err := bucket.Delete(ctx); err != nil {
		t.Errorf("Failed to delete Bucket %s", bucketName)
	}
}

func getBucketHandle(ctx context.Context, t *testing.T, bucketName, project string) *storage.BucketHandle {
	t.Helper()
	// Prow sticks the project in this key
	if project == "" {
		t.Fatalf("failed to find %q in envvars", ProwProjectKey)
	}
	opt := option.WithQuotaProject(project)
	client, err := storage.NewClient(ctx, opt)
	if err != nil {
		t.Fatalf("failed to create pubsub client, %s", err.Error())
	}
	return client.Bucket(bucketName)
}

func NotificationExists(t *testing.T, bucketName, notificationID string) bool {
	t.Helper()
	ctx := context.Background()
	project := GetEnvOrFail(t, ProwProjectKey)
	opt := option.WithQuotaProject(project)
	client, err := storage.NewClient(ctx, opt)
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

	if _, ok := notifications[notificationID]; ok {
		return true
	}
	return false

}
