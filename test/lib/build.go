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

	v1 "k8s.io/api/core/v1"

	"google.golang.org/api/option"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1/v2"
	"github.com/google/knative-gcp/test/lib/resources"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildConfig struct {
	SinkGVK            metav1.GroupVersionKind
	BuildName          string
	SinkName           string
	ServiceAccountName string
}

func MakeBuildOrDie(client *Client, config BuildConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1.CloudBuildSourceOption, 0)
	so = append(so, reconcilertestingv1.WithCloudBuildSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1.WithCloudBuildSourceServiceAccount(config.ServiceAccountName))
	build := reconcilertestingv1.NewCloudBuildSource(config.BuildName, client.Namespace, so...)
	client.CreateBuildOrFail(build)
	// CloudBuildSource may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.BuildName, CloudBuildSourceV1TypeMeta)
}

func MakeBuildV1beta1OrDie(client *Client, config BuildConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1beta1.CloudBuildSourceOption, 0)
	so = append(so, reconcilertestingv1beta1.WithCloudBuildSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1beta1.WithCloudBuildSourceServiceAccount(config.ServiceAccountName))
	build := reconcilertestingv1beta1.NewCloudBuildSource(config.BuildName, client.Namespace, so...)
	client.CreateBuildV1beta1OrFail(build)
	// CloudBuildSource source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.BuildName, CloudBuildSourceV1beta1TypeMeta)
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
	project := GetEnvOrFail(t, ProwProjectKey)
	opt := option.WithQuotaProject(project)
	client, err := cloudbuild.NewClient(ctx, opt)
	if err != nil {
		t.Fatalf("failed to create cloud build client, %s", err.Error())
	}
	defer client.Close()

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
