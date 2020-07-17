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

	scheduler "cloud.google.com/go/scheduler/apiv1"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
)

// CreateFn is a factory function to create a Scheduler client.
type CreateFn func(ctx context.Context, opts ...option.ClientOption) (Client, error)

// NewClient creates a new wrapped Scheduler client.
func NewClient(ctx context.Context, opts ...option.ClientOption) (Client, error) {
	client, err := scheduler.NewCloudSchedulerClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &schedulerClient{
		client: client,
	}, nil
}

// schedulerClient wraps scheduler.CloudSchedulerClient. Is the client that will be used everywhere except unit tests.
type schedulerClient struct {
	client *scheduler.CloudSchedulerClient
}

// Verify that it satisfies the scheduler.CloudSchedulerClient interface.
var _ Client = &schedulerClient{}

// Close implements scheduler.CloudSchedulerClient.Close
func (c *schedulerClient) Close() error {
	return c.client.Close()
}

// CreateJob implements scheduler.CloudSchedulerClient.CreateJob
func (c *schedulerClient) CreateJob(ctx context.Context, req *schedulerpb.CreateJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error) {
	return c.client.CreateJob(ctx, req, opts...)
}

// UpdateJob implements scheduler.CloudSchedulerClient.UpdateJobRequest
func (c *schedulerClient) UpdateJob(ctx context.Context, req *schedulerpb.UpdateJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error) {
	return c.client.UpdateJob(ctx, req, opts...)
}

// DeleteJob implements scheduler.CloudSchedulerClient.DeleteJob
func (c *schedulerClient) DeleteJob(ctx context.Context, req *schedulerpb.DeleteJobRequest, opts ...gax.CallOption) error {
	return c.client.DeleteJob(ctx, req, opts...)
}

// GetJob implements scheduler.CloudSchedulerClient.GetJob
func (c *schedulerClient) GetJob(ctx context.Context, req *schedulerpb.GetJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error) {
	return c.client.GetJob(ctx, req, opts...)
}
