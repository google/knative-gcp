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
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1/v2"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildConfig struct {
	SinkGVK            metav1.GroupVersionKind
	BuildName          string
	SinkName           string
	ServiceAccountName string
	Options            []kngcptesting.CloudBuildSourceOption
}

func MakeBuildOrDie(client *Client, config BuildConfig) {
	client.T.Helper()
	so := config.Options
	so = append(so, kngcptesting.WithCloudBuildSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, kngcptesting.WithCloudBuildSourceServiceAccount(config.ServiceAccountName))
	build := kngcptesting.NewCloudBuildSource(config.BuildName, client.Namespace, so...)
	client.CreateBuildOrFail(build)

	client.Core.WaitForResourceReadyOrFail(config.BuildName, CloudBuildSourceTypeMeta)
}

func MakeBuildTargetJobOrDie(client *Client, images, targetName, eventType string) {
	client.T.Helper()
	job := resources.BuildTargetJob(targetName, []v1.EnvVar{
		{
			Name:  "TYPE",
			Value: eventType,
		},
		{
			Name:  "IMAGES",
			Value: images,
		}, {
			Name:  "TIME",
			Value: "6m",
		}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}

func BuildWithConfigFile(t *testing.T, imageName string) string {
	ctx := context.Background()
	client, err := cloudbuild.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create cloud build client, %s", err.Error())
	}
	defer client.Close()

	project := os.Getenv(ProwProjectKey)
	image := CloudBuildImage(project, imageName)
	build := new(cloudbuildpb.Build)
	build.Steps = append(build.Steps, &cloudbuildpb.BuildStep{
		Name: "gcr.io/cloud-builders/docker",
		Args: []string{"build", "-t", image, "."},
	})
	build.Images = []string{image}

	req := &cloudbuildpb.CreateBuildRequest{
		Build:     build,
		ProjectId: project,
	}

	op, err := client.CreateBuild(ctx, req)
	if err != nil {
		t.Fatalf("failed to a build with the specified configuration, %s", err.Error())
	}

	_, err = op.Wait(ctx)
	if err != nil {
		t.Logf("failed to build docker image, %s", err.Error())
	}

	metadata, err := op.Metadata()
	if err != nil {
		t.Fatalf("failed to get operation metedata, %s", err.Error())
	}
	return metadata.Build.Id
}

func CloudBuildImage(project, imageName string) string {
	return fmt.Sprintf("gcr.io/%s/%s", project, imageName)
}
