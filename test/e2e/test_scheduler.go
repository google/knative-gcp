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
	"encoding/json"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	kngcpresources "github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

// SmokeCloudSchedulerSourceSetup tests if a CloudSchedulerSource object can be created and be made ready.
func SmokeCloudSchedulerSourceSetup(t *testing.T) {
	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	sName := "scheduler-test"

	scheduler := kngcptesting.NewCloudSchedulerSource(sName, client.Namespace,
		kngcptesting.WithCloudSchedulerSourceLocation("us-central1"),
		kngcptesting.WithCloudSchedulerSourceData("my test data"),
		kngcptesting.WithCloudSchedulerSourceSchedule("* * * * *"),
		kngcptesting.WithCloudSchedulerSourceSink(lib.ServiceGVK, "event-display"),
	)

	client.CreateSchedulerOrFail(scheduler)
	client.Core.WaitForResourceReadyOrFail(sName, lib.CloudSchedulerSourceTypeMeta)
}

// CloudSchedulerSourceWithTargetTestImpl injects a scheduler event and checks if it is in the
// log of the receiver.
func CloudSchedulerSourceWithTargetTestImpl(t *testing.T) {
	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	// Create an Addressable to receive scheduler events
	data := "my test data"
	targetName := "event-display"
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
	client.CreateJobOrFail(job, lib.WithServiceForJob(targetName))

	// Create a scheduler
	sName := "scheduler-test"

	scheduler := kngcptesting.NewCloudSchedulerSource(sName, client.Namespace,
		kngcptesting.WithCloudSchedulerSourceLocation("us-central1"),
		kngcptesting.WithCloudSchedulerSourceData(data),
		kngcptesting.WithCloudSchedulerSourceSchedule("* * * * *"),
		kngcptesting.WithCloudSchedulerSourceSink(lib.ServiceGVK, targetName),
	)

	client.CreateSchedulerOrFail(scheduler)
	client.Core.WaitForResourceReadyOrFail(sName, lib.CloudSchedulerSourceTypeMeta)

	msg, err := client.WaitUntilJobDone(client.Namespace, targetName)
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
			if logs, err := client.LogsFor(client.Namespace, sName, lib.CloudSchedulerSourceTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("scheduler log: %+v", logs)
			}

			// Log the output of the target job pods
			if logs, err := client.LogsFor(client.Namespace, targetName, lib.JobTypeMeta); err != nil {
				t.Error(err)
			} else {
				t.Logf("addressable job: %+v", logs)
			}
			t.Fail()
		}
	}
}
