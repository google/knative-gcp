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
	kngcpresources "github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeSchedulerOrDie(client *Client,
	gvk metav1.GroupVersionKind, schedulerName, data, sinkName, pubsubServiceAccount string,
	so ...kngcptesting.CloudSchedulerSourceOption,
) {
	client.T.Helper()
	so = append(so, kngcptesting.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceData(data))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSink(gvk, sinkName))
	so = append(so, kngcptesting.WithCloudSchedulerSourceGCPServiceAccount(pubsubServiceAccount))
	scheduler := kngcptesting.NewCloudSchedulerSource(schedulerName, client.Namespace, so...)

	client.CreateSchedulerOrFail(scheduler)
	client.Core.WaitForResourceReadyOrFail(schedulerName, CloudSchedulerSourceTypeMeta)
}

func MakeSchedulerJobOrDie(client *Client, data, targetName, eventType string) {
	client.T.Helper()
	job := resources.SchedulerTargetJob(targetName, []v1.EnvVar{
		{
			Name:  "TIME",
			Value: "6m",
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
			Value: eventType,
		},
	})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}
