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
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

// GenerateJobName generates a job name like this: projects/PROJECT_ID/locations/LOCATION_ID/jobs/JOB_ID.
func GenerateJobName(scheduler *v1alpha1.Scheduler) string {
	return fmt.Sprintf("projects/%s/locations/%s/jobs/cre-scheduler-%s", scheduler.Status.ProjectID, scheduler.Spec.Location, string(scheduler.UID))
}
