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

package trigger

import (
	"context"

	"google.golang.org/api/option"
)

// CreateFn is a factory function to create a Trigger client.
type CreateFn func(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error)

// NewClient creates a new wrapped trigger client.
func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (Client, error) {
	client, err := fakeNewClient()
	if err != nil {
		return nil, err
	}
	return &triggerClient{
		client: client,
	}, nil
}

func fakeNewClient() (*FakeTriggerClient, error) {
	return nil, nil
}

// triggerClient wraps triggers.Client. Is the client that will be used everywhere except unit tests.
type triggerClient struct {
	client *FakeTriggerClient
}

//  A placeholder struct to be replaced by one provided by EventFlow
type FakeTriggerClient struct{}

func (c *FakeTriggerClient) Close() error {
	return nil
}

func (c *FakeTriggerClient) Trigger(id string) *FakeTrigger {
	return nil
}

func (c *FakeTriggerClient) CreateTrigger(ctx context.Context, id string, sourceType string, filters map[string]string) (*FakeTrigger, error) {
	return nil, nil
}

// Verify that it satisfies the triggers.Client interface.
var _ Client = &triggerClient{}

// Close implements triggers.Client.Close
func (c *triggerClient) Close() error {
	return c.client.Close()
}

// Trigger implements triggers.Client.Trigger
func (c *triggerClient) Trigger(id string) Trigger {
	return &trigger{trigger: c.client.Trigger(id)}
}

// CreateTrigger implements triggers.Client.CreateTrigger
func (c *triggerClient) CreateTrigger(ctx context.Context, id string, sourceType string, filters map[string]string) (Trigger, error) {
	t, err := c.client.CreateTrigger(ctx, id, sourceType, filters)
	if err != nil {
		return nil, err
	}
	return &trigger{trigger: t}, nil
}
