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
	v1 "k8s.io/api/core/v1"

	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
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

func MakeBuildTargetJobOrDie(client *Client, source, targetName, eventType string) {
	client.T.Helper()
	job := resources.BuildTargetJob(targetName, []v1.EnvVar{
		{
			Name:  "TYPE",
			Value: eventType,
		},
		{
			Name:  "SOURCE",
			Value: source,
		}, {
			Name:  "TIME",
			Value: "6m",
		}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}
