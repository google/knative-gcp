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
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	v1 "k8s.io/api/core/v1"
)

func MakeAuditLogsOrDie(client *Client,
	auditlogsName, methodName, project, resourceName, serviceName, targetName, pubsubServiceAccount string,
	so ...kngcptesting.CloudAuditLogsSourceOption,
) {
	so = append(so, kngcptesting.WithCloudAuditLogsSourceServiceName(serviceName))
	so = append(so, kngcptesting.WithCloudAuditLogsSourceMethodName(methodName))
	so = append(so, kngcptesting.WithCloudAuditLogsSourceProject(project))
	so = append(so, kngcptesting.WithCloudAuditLogsSourceResourceName(resourceName))
	so = append(so, kngcptesting.WithCloudAuditLogsSourceSink(ServiceGVK, targetName))
	so = append(so, kngcptesting.WithCloudAuditLogsSourceGCPServiceAccount(pubsubServiceAccount))
	eventsAuditLogs := kngcptesting.NewCloudAuditLogsSource(auditlogsName, client.Namespace, so...)
	client.CreateAuditLogsOrFail(eventsAuditLogs)

	client.Core.WaitForResourceReadyOrFail(auditlogsName, CloudAuditLogsSourceTypeMeta)
}

func MakeAuditLogsJobOrDie(client *Client, methodName, project, resourceName, serviceName, targetName string) {
	job := resources.AuditLogsTargetJob(targetName, []v1.EnvVar{{
		Name:  "SERVICENAME",
		Value: serviceName,
	}, {
		Name:  "METHODNAME",
		Value: methodName,
	}, {
		Name:  "RESOURCENAME",
		Value: resourceName,
	}, {
		Name:  "TYPE",
		Value: v1alpha1.CloudAuditLogsSourceEvent,
	}, {
		Name:  "SOURCE",
		Value: v1alpha1.CloudAuditLogsSourceEventSource(serviceName, fmt.Sprintf("projects/%s", project)),
	}, {
		Name:  "SUBJECT",
		Value: resourceName,
	}, {
		Name:  "TIME",
		Value: "360",
	}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}
