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

package scheduler

import (
	"context"

	"github.com/googleapis/gax-go/v2"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
)

// Client matches the interface exposed by scheduler.Client
// see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient
type Client interface {
	// Close see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient.Close
	Close() error
	// CreateJob see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient.CreateJob
	CreateJob(ctx context.Context, req *schedulerpb.CreateJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error)
	// UpdateJob see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient.UpdateJob
	UpdateJob(ctx context.Context, req *schedulerpb.UpdateJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error)
	// DeleteJob see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient.DeleteJob
	DeleteJob(ctx context.Context, req *schedulerpb.DeleteJobRequest, opts ...gax.CallOption) error
	// GetJob see https://godoc.org/cloud.google.com/go/scheduler/apiv1#CloudSchedulerClient.GetJob
	GetJob(ctx context.Context, req *schedulerpb.GetJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error)
}
