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
	"testing"
	"time"

	reconcilertestingv1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1"
	reconcilertestingv1alpha1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1alpha1"
	reconcilertestingv1beta1 "github.com/google/knative-gcp/pkg/reconciler/testing/v1beta1"

	"google.golang.org/api/option"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/gclient/scheduler"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

type SchedulerConfig struct {
	SinkGVK            metav1.GroupVersionKind
	SchedulerName      string
	Data               string
	SinkName           string
	ServiceAccountName string
}

func MakeSchedulerOrDie(client *Client, config SchedulerConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1.CloudSchedulerSourceOption, 0)
	so = append(so, reconcilertestingv1.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, reconcilertestingv1.WithCloudSchedulerSourceData(config.Data))
	so = append(so, reconcilertestingv1.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, reconcilertestingv1.WithCloudSchedulerSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1.WithCloudSchedulerSourceServiceAccount(config.ServiceAccountName))
	scheduler := reconcilertestingv1.NewCloudSchedulerSource(config.SchedulerName, client.Namespace, so...)

	client.CreateSchedulerOrFail(scheduler)
	// Scheduler source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.SchedulerName, CloudSchedulerSourceV1TypeMeta)
}

func MakeSchedulerV1beta1OrDie(client *Client, config SchedulerConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1beta1.CloudSchedulerSourceOption, 0)
	so = append(so, reconcilertestingv1beta1.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, reconcilertestingv1beta1.WithCloudSchedulerSourceData(config.Data))
	so = append(so, reconcilertestingv1beta1.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, reconcilertestingv1beta1.WithCloudSchedulerSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1beta1.WithCloudSchedulerSourceServiceAccount(config.ServiceAccountName))
	scheduler := reconcilertestingv1beta1.NewCloudSchedulerSource(config.SchedulerName, client.Namespace, so...)

	client.CreateSchedulerV1beta1OrFail(scheduler)
	// Scheduler source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.SchedulerName, CloudSchedulerSourceV1beta1TypeMeta)
}

func MakeSchedulerV1alpha1OrDie(client *Client, config SchedulerConfig) {
	client.T.Helper()
	so := make([]reconcilertestingv1alpha1.CloudSchedulerSourceOption, 0)
	so = append(so, reconcilertestingv1alpha1.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, reconcilertestingv1alpha1.WithCloudSchedulerSourceData(config.Data))
	so = append(so, reconcilertestingv1alpha1.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, reconcilertestingv1alpha1.WithCloudSchedulerSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, reconcilertestingv1alpha1.WithCloudSchedulerSourceServiceAccount(config.ServiceAccountName))
	scheduler := reconcilertestingv1alpha1.NewCloudSchedulerSource(config.SchedulerName, client.Namespace, so...)

	client.CreateSchedulerV1alpha1OrFail(scheduler)
	// Scheduler source may not be ready within the 2 min timeout in WaitForResourceReadyOrFail function.
	time.Sleep(resources.WaitExtraSourceReadyTime)
	client.Core.WaitForResourceReadyOrFail(config.SchedulerName, CloudSchedulerSourceV1alpha1TypeMeta)
}

func MakeSchedulerJobOrDie(client *Client, data, targetName, eventType string) {
	client.T.Helper()
	job := resources.SchedulerTargetJob(targetName, []v1.EnvVar{
		{
			Name:  "TIME",
			Value: "6m",
		},
		{
			Name:  "DATA",
			Value: schedulerEventPayload(data),
		},
		{
			Name:  "TYPE",
			Value: eventType,
		},
	})
	client.CreateJobOrFail(job, WithServiceForJob(targetName))
}

func schedulerEventPayload(customData string) string {
	jd := &schemasv1.SchedulerJobData{CustomData: []byte(customData)}
	b, _ := json.Marshal(jd)
	return string(b)
}

func SchedulerJobExists(t *testing.T, jobName string) bool {
	t.Helper()
	ctx := context.Background()
	project := GetEnvOrFail(t, ProwProjectKey)
	opt := option.WithQuotaProject(project)
	client, err := scheduler.NewClient(ctx, opt)
	if err != nil {
		t.Fatalf("failed to create scheduler client, %s", err.Error())
	}
	defer client.Close()

	_, err = client.GetJob(ctx, &schedulerpb.GetJobRequest{Name: jobName})
	if err != nil {
		st, ok := gstatus.FromError(err)
		if !ok {
			t.Fatalf("Failed from CloudSchedulerSource client while retrieving CloudSchedulerSource job %s with error %s", jobName, err.Error())
		}
		if st.Code() == codes.NotFound {
			return false
		}

		t.Fatalf("Failed from CloudSchedulerSource client while retrieving CloudSchedulerSource job %s with error %s with status code %s", jobName, err.Error(), st.Code())
	}
	return true
}
