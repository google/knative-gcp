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

package testing

import (
	"context"
	"google.golang.org/api/option"

	"github.com/google/knative-gcp/pkg/gclient/scheduler"
	"github.com/googleapis/gax-go/v2"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1"
)

// TestClientCreator returns a scheduler.CreateFn used to construct the test Scheduler client.
func TestClientCreator(data *TestClientData) scheduler.CreateFn {
	if data == nil {
		data = &TestClientData{}
	}
	if data.createClientErr != nil {
		return func(_ context.Context, _ ...option.ClientOption) (scheduler.Client, error) {
			return nil, data.createClientErr
		}
	}

	return func(_ context.Context, _ ...option.ClientOption) (scheduler.Client, error) {
		return &testClient{
			data: *data,
		}, nil
	}
}

// TestClientData is the data used to configure the test Scheduler client.
type TestClientData struct {
	createClientErr error
	createJobErr    error
	deleteJobErr    error
	getJobErr       error
	closeErr        error
}

// testClient is the test Scheduler client.
type testClient struct {
	data TestClientData
}

// Verify that it satisfies the scheduler.Client interface.
var _ scheduler.Client = &testClient{}

// Close implements client.Close
func (c *testClient) Close() error {
	return c.data.closeErr
}

// CreateJob implements client.CreateJob
func (c *testClient) CreateJob(ctx context.Context, req *schedulerpb.CreateJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error) {
	if c.data.createJobErr != nil {
		return nil, c.data.createJobErr
	}
	return &schedulerpb.Job{
		Name: "jobName",
	}, nil
}

// CreateJob implements client.DeleteJob
func (c *testClient) DeleteJob(ctx context.Context, req *schedulerpb.DeleteJobRequest, opts ...gax.CallOption) error {
	return c.data.deleteJobErr
}

// GetJob implements client.GetJob
func (c *testClient) GetJob(ctx context.Context, req *schedulerpb.GetJobRequest, opts ...gax.CallOption) (*schedulerpb.Job, error) {
	if c.data.deleteJobErr != nil {
		return nil, c.data.deleteJobErr
	}
	return &schedulerpb.Job{
		Name: "jobName",
	}, nil
}
