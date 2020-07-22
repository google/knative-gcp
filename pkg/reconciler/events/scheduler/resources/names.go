/*
Copyright 2019 Google LLC

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

package resources

import (
	"fmt"
	"strings"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	"github.com/google/knative-gcp/pkg/utils/naming"
)

const (
	JobPrefix = "jobs/cre-scheduler"
)

// GenerateJobName generates a job name like this: projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID.
func GenerateJobName(scheduler *v1.CloudSchedulerSource) string {
	return fmt.Sprintf("projects/%s/locations/%s/%s-%s", scheduler.Status.ProjectID, scheduler.Spec.Location, JobPrefix, string(scheduler.UID))
}

// ExtractParentName extracts the parent from the job name.
// Example: given projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID, returns projects/PROJECT_ID/locations/LOCATION_ID.
func ExtractParentName(jobName string) string {
	return jobName[0:strings.LastIndex(jobName, "/jobs/")]
}

// ExtractJobID extracts the JobID from the job name.
// Example: given projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID, returns jobs/JOB_ID.
func ExtractJobID(jobName string) string {
	return jobName[strings.LastIndex(jobName, "/jobs/")+1:]
}

// GenerateTopicName generates a topic name for the scheduler. This refers to the underlying Pub/Sub topic, and not our
// Topic resource.
func GenerateTopicName(scheduler *v1.CloudSchedulerSource) string {
	return naming.TruncatedPubsubResourceName("cre-src", scheduler.Namespace, scheduler.Name, scheduler.UID)
}

// GeneratePubSubTarget generates a topic name for the PubsubTarget used to create the CloudSchedulerSource job.
func GeneratePubSubTargetTopic(scheduler *v1.CloudSchedulerSource, topic string) string {
	return fmt.Sprintf("projects/%s/topics/%s", scheduler.Status.ProjectID, topic)
}
