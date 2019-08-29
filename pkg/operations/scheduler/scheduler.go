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

package operations

import (
	"context"
	"fmt"

	"cloud.google.com/go/compute/metadata"

	schedulerv1 "cloud.google.com/go/scheduler/apiv1"
)

type SchedulerOps struct {
	// Environment variable containing project id.
	Project string `envconfig:"PROJECT_ID"`

	// client is the GCS client used by Ops.
	client *schedulerv1.CloudSchedulerClient
}

func (s *SchedulerOps) CreateClient(ctx context.Context) error {
	if s.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			return err
		}
		s.Project = project
	}

	client, err := schedulerv1.NewCloudSchedulerClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Scheduler client: %s", err)
	}

	s.client = client
	return nil
}
