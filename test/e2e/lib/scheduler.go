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
	"os"
	"testing"

	"github.com/google/knative-gcp/pkg/gclient/scheduler"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	"google.golang.org/api/option"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
	"google.golang.org/grpc/codes"
	gstatus "google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SchedulerConfig struct {
	SinkGVK            metav1.GroupVersionKind
	SchedulerName      string
	Data               string
	SinkName           string
	ServiceAccountName string
	Options            []kngcptesting.CloudSchedulerSourceOption
}

func MakeSchedulerOrDie(client *Client, config SchedulerConfig) {
	client.T.Helper()
	so := config.Options
	so = append(so, kngcptesting.WithCloudSchedulerSourceLocation("us-central1"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceData(config.Data))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSchedule("* * * * *"))
	so = append(so, kngcptesting.WithCloudSchedulerSourceSink(config.SinkGVK, config.SinkName))
	so = append(so, kngcptesting.WithCloudSchedulerSourceServiceAccount(config.ServiceAccountName))
	scheduler := kngcptesting.NewCloudSchedulerSource(config.SchedulerName, client.Namespace, so...)

	client.CreateSchedulerOrFail(scheduler)
	client.Core.WaitForResourceReadyOrFail(config.SchedulerName, CloudSchedulerSourceTypeMeta)
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
	project := os.Getenv(ProwProjectKey)
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
