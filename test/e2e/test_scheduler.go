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

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"knative.dev/pkg/test/helpers"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/lib"
	"github.com/google/knative-gcp/test/lib/resources"
)

// SmokeCloudSchedulerSourceTestHelper tests if a CloudSchedulerSource object can be created to ready state.
func SmokeCloudSchedulerSourceTestHelper(t *testing.T, authConfig lib.AuthConfig, cloudSchedulerSourceVersion string) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	// Create an Addressable to receive scheduler events
	data := helpers.AppendRandomString("smoke-scheduler-source")
	// Create the target and scheduler
	schedulerName := helpers.AppendRandomString("scheduler")
	svcName := "event-display"

	schedulerConfig := lib.SchedulerConfig{
		SinkGVK:            lib.ServiceGVK,
		SchedulerName:      schedulerName,
		Data:               data,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	}

	if cloudSchedulerSourceVersion == "v1beta1" {
		lib.MakeSchedulerOrDie(client, schedulerConfig)
	} else if cloudSchedulerSourceVersion == "v1" {
		lib.MakeSchedulerOrDie(client, schedulerConfig)
	} else {
		t.Fatalf("SmokeCloudSchedulerSourceTestHelper does not support CloudSchedulerSource version: %v", cloudSchedulerSourceVersion)
	}
}

// SmokeCloudSchedulerSourceWithDeletionTestImpl tests if a CloudSchedulerSource object can be created to ready state and delete a CloudSchedulerSource resource and its underlying resources..
func SmokeCloudSchedulerSourceWithDeletionTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	// Create an Addressable to receive scheduler events
	data := helpers.AppendRandomString("smoke-scheduler-source")
	// Create the target and scheduler
	schedulerName := helpers.AppendRandomString("scheduler")
	svcName := "event-display"
	lib.MakeSchedulerOrDie(client, lib.SchedulerConfig{
		SinkGVK:            lib.ServiceGVK,
		SchedulerName:      schedulerName,
		Data:               data,
		SinkName:           svcName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	createdScheduler := client.GetSchedulerOrFail(schedulerName)

	topicID := createdScheduler.Status.TopicID
	subID := createdScheduler.Status.SubscriptionID
	jobName := createdScheduler.Status.JobName

	createdJobExists := lib.SchedulerJobExists(t, jobName)
	if !createdJobExists {
		t.Errorf("Expected job%q to exist", jobName)
	}

	createdTopicExists := lib.TopicExists(t, topicID)
	if !createdTopicExists {
		t.Errorf("Expected topic%q to exist", topicID)
	}

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}
	client.DeleteSchedulerOrFail(schedulerName)
	//Wait for 120 seconds for topic, subscription and job to get deleted in gcp
	time.Sleep(resources.WaitDeletionTime)

	deletedJobExists := lib.SchedulerJobExists(t, jobName)
	if deletedJobExists {
		t.Errorf("Expected job%q to get deleted", jobName)
	}

	deletedTopicExists := lib.TopicExists(t, topicID)
	if deletedTopicExists {
		t.Errorf("Expected topic %q to get deleted", topicID)
	}

	deletedSubExists := lib.SubscriptionExists(t, subID)
	if deletedSubExists {
		t.Errorf("Expected subscription %q to get deleted", subID)
	}
}

// CloudSchedulerSourceWithTargetTestImpl injects a scheduler event and checks if it is in the
// log of the receiver.
func CloudSchedulerSourceWithTargetTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	// Create an Addressable to receive scheduler events
	data := helpers.AppendRandomString("scheduler-source-with-target")
	// Create the target and scheduler
	schedulerName := helpers.AppendRandomString("scheduler")
	targetName := helpers.AppendRandomString(schedulerName + "-target")
	lib.MakeSchedulerJobOrDie(client, data, targetName, schemasv1.CloudSchedulerJobExecutedEventType)
	lib.MakeSchedulerOrDie(client, lib.SchedulerConfig{
		SinkGVK:            lib.ServiceGVK,
		SchedulerName:      schedulerName,
		Data:               data,
		SinkName:           targetName,
		ServiceAccountName: authConfig.ServiceAccountName,
	})

	msg, err := client.WaitUntilJobDone(ctx, client.Namespace, targetName)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Last termination message => %s", msg)
	if msg != "" {
		out := &lib.TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
		}
		if !out.Success {
			// Log the output of scheduler pods
			if logs, err := client.LogsFor(ctx, client.Namespace, schedulerName, lib.CloudSchedulerSourceV1TypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("scheduler log: %+v", logs)
			}

			// Log the output of the target job pods
			if logs, err := client.LogsFor(ctx, client.Namespace, targetName, lib.JobTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("addressable job: %+v", logs)
			}
			t.Fail()
		}
	}
}
