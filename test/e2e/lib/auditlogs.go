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
	reconcilertestingv1alpha1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1alpha1"
	reconcilertestingv1beta1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1beta1"

	"google.golang.org/api/option"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/grpc/codes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

const (
	PubSubServiceName           = "pubsub.googleapis.com"
	PubSubCreateTopicMethodName = "google.pubsub.v1.Publisher.CreateTopic"
)

type AuditLogsConfig struct {
	SinkGVK            metav1.GroupVersionKind
	SinkName           string
	AuditlogsName      string
	MethodName         string
	Project            string
	ResourceName       string
	ServiceName        string
	ServiceAccountName string
}

func MakeAuditLogsOrDie(client *Client, config AuditLogsConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1.CloudAuditLogsSourceOption, 0)
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceServiceName(config.ServiceName))
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceMethodName(config.MethodName))
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceProject(config.Project))
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceResourceName(config.ResourceName))
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1.WithCloudAuditLogsSourceServiceAccount(config.ServiceAccountName))
	eventsAuditLogs := reconcilertestingv1.NewCloudAuditLogsSource(config.AuditlogsName, client.Namespace, so...)
	client.CreateAuditLogsOrFail(eventsAuditLogs)
	// AuditLog source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.AuditlogsName, CloudAuditLogsSourceV1TypeMeta)
}

func MakeAuditLogsV1beta1OrDie(client *Client, config AuditLogsConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1beta1.CloudAuditLogsSourceOption, 0)
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceServiceName(config.ServiceName))
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceMethodName(config.MethodName))
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceProject(config.Project))
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceResourceName(config.ResourceName))
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1beta1.WithCloudAuditLogsSourceServiceAccount(config.ServiceAccountName))
	eventsAuditLogs := reconcilertestingv1beta1.NewCloudAuditLogsSource(config.AuditlogsName, client.Namespace, so...)
	client.CreateAuditLogsV1beta1OrFail(eventsAuditLogs)
	// AuditLog source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.AuditlogsName, CloudAuditLogsSourceV1beta1TypeMeta)
}

func MakeAuditLogsV1alpha1OrDie(client *Client, config AuditLogsConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1alpha1.CloudAuditLogsSourceOption, 0)
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceServiceName(config.ServiceName))
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceMethodName(config.MethodName))
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceProject(config.Project))
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceResourceName(config.ResourceName))
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1alpha1.WithCloudAuditLogsSourceServiceAccount(config.ServiceAccountName))
	eventsAuditLogs := reconcilertestingv1alpha1.NewCloudAuditLogsSource(config.AuditlogsName, client.Namespace, so...)
	client.CreateAuditLogsV1alpha1OrFail(eventsAuditLogs)
	// AuditLog source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.AuditlogsName, CloudAuditLogsSourceV1alpha1TypeMeta)
}

func MakeAuditLogsJobOrDie(client *Client, methodName, project, resourceName, serviceName, targetName, eventType string) {
	client.T.Helper()
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
		Value: eventType,
	}, {
		Name:  "SOURCE",
		Value: schemasv1.CloudAuditLogsEventSource(fmt.Sprintf("projects/%s", project), "activity"),
	}, {
		Name:  "SUBJECT",
		Value: schemasv1.CloudAuditLogsEventSubject(serviceName, resourceName),
	}, {
		Name:  "TIME",
		Value: "6m",
	}})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}

func StackdriverSinkExists(t *testing.T, sinkID string) bool {
	t.Helper()
	ctx := context.Background()
	project := GetEnvOrFail(t, ProwProjectKey)
	opt := option.WithQuotaProject(project)
	client, err := logadmin.NewClient(ctx, project, opt)
	if err != nil {
		t.Fatalf("failed to create LogAdmin client, %s", err.Error())
	}
	defer client.Close()

	_, err = client.Sink(ctx, sinkID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false
		}

		t.Fatalf("Failed from LogAdmin client while retrieving StackdriverSink %s with error %s", sinkID, err.Error())
	}
	return true
}
