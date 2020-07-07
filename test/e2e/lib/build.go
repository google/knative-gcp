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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"

	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	"google.golang.org/api/cloudbuild/v1"
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
	project := os.Getenv(ProwProjectKey)
	cloudbuildService, err := cloudbuild.NewService(ctx)
	if err != nil {
		t.Fatalf("failed to create cloud build service, %s", err.Error())
	}
	build := new(cloudbuild.Build)
	image := CloudBuildImage(project, imageName)
	build.Steps = append(build.Steps, &cloudbuild.BuildStep{
		Name: "gcr.io/cloud-builders/docker",
		Args: []string{"build", "-t", image, "."},
	})
	build.Images = []string{image}
	projectsBuildsCreateCall := cloudbuildService.Projects.Builds.Create(project, build)
	operation, err := projectsBuildsCreateCall.Do()
	if err != nil {
		t.Fatalf("failed to build docker image, %s", err.Error())
	}
	bom, err := extractOperationMetadata(operation)
	if err != nil {
		t.Fatalf("failed to extract operation metedata")
	}
	if bom.Build.Id == "" {
		t.Fatalf("No build ID returned in BuildOperationMetadata")
	}
	return bom.Build.Id
}

func extractOperationMetadata(op *cloudbuild.Operation) (*cloudbuild.BuildOperationMetadata, error) {
	mdJSON, err := json.Marshal(op.Metadata)
	if err != nil {
		return nil, err
	}
	bom := new(cloudbuild.BuildOperationMetadata)
	err = json.Unmarshal(mdJSON, bom)
	if err != nil {
		return nil, err
	}
	if bom.Build == nil {
		return nil, errors.New("no build found in assumed BuildOperationMetadata")
	}
	return bom, nil
}

func CloudBuildImage(project, imageName string) string {
	return fmt.Sprintf("gcr.io/%s/%s", project, imageName)
}


