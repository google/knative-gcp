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
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcpresources "github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	v1 "k8s.io/api/core/v1"
)

func MakeSchedulerOrDie(client *Client,
	sName, data, targetName, pubsubServiceAccount string,
	so ...kngcptesting.CloudSchedulerSourceOption,
) {
	so = append(so, kngcptesting.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceData(data))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSink(ServiceGVK, targetName))
	so = append(so, kngcptesting.WithCloudSchedulerSourceGCPServiceAccount(pubsubServiceAccount))
	scheduler := kngcptesting.NewCloudSchedulerSource(sName, client.Namespace, so...)

	client.CreateSchedulerOrFail(scheduler)
	client.Core.WaitForResourceReadyOrFail(sName, CloudSchedulerSourceTypeMeta)
}

func MakeSchedulerJobOrDie(client *Client, data, targetName string) {
	job := resources.SchedulerJob(targetName, []v1.EnvVar{
		{
			Name:  "TIME",
			Value: "360",
		},
		{
			Name:  "SUBJECT_PREFIX",
			Value: kngcpresources.JobPrefix,
		},
		{
			Name:  "DATA",
			Value: data,
		},
		{
			Name:  "TYPE",
			Value: v1alpha1.CloudSchedulerSourceExecute,
		},
	})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}
