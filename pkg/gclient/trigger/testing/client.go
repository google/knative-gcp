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

	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	"github.com/google/knative-gcp/pkg/gclient/trigger"
	"google.golang.org/api/option"
)

// TestClientCreator returns a triggers.CreateFn used to construct the test Trigger client.
func TestClientCreator(value interface{}) trigger.CreateFn {
	var data TestClientData
	var ok bool
	if data, ok = value.(TestClientData); !ok {
		data = TestClientData{}
	}
	if data.CreateClientErr != nil {
		return func(ctx context.Context, projectID string, opts ...option.ClientOption) (trigger.Client, error) {
			return nil, data.CreateClientErr
		}
	}

	return func(ctx context.Context, projectID string, opts ...option.ClientOption) (trigger.Client, error) {
		return &testClient{
			data: data,
		}, nil
	}
}

// TestClientData is the data used to configure the test Trigger client.
type TestClientData struct {
	CreateClientErr  error
	CloseErr         error
	CreateTriggerErr error
	TriggerData      TestTriggerData
	HandleData       testiam.TestHandleData
}

// testClient is a test Trigger client.
type testClient struct {
	data TestClientData
}

// Verify that it satisfies the trigger.Client interface.
var _ trigger.Client = &testClient{}

// Close implements client.Close
func (c *testClient) Close() error {
	return c.data.CloseErr
}

// Trigger implements triggers.Client.Trigger
func (c *testClient) Trigger(id string) trigger.Trigger {
	return &testTrigger{data: c.data.TriggerData, handleData: c.data.HandleData, id: id}
}

// CreateTrigger implements triggers.Client.CreateTrigger
func (c *testClient) CreateTrigger(ctx context.Context, id string, sourceType string, filters map[string]string) (trigger.Trigger, error) {
	return &testTrigger{data: c.data.TriggerData, handleData: c.data.HandleData, id: id}, c.data.CreateTriggerErr
}
