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

	ciam "cloud.google.com/go/iam"
	"github.com/google/knative-gcp/pkg/gclient/iam"
)

// pubsubTopic wraps eventflow.Trigger. Is the topic that will be used everywhere except unit tests.
type trigger struct {
	trigger *FakeTrigger
}

// A placeholder struct to be replaced by one provided by EventFlow
type FakeTrigger struct {
}

func (f FakeTrigger) ID() string {
	return "ID"
}

func (f FakeTrigger) Delete(ctx context.Context) error {
	return nil
}

func (f FakeTrigger) Exists(ctx context.Context) (bool, error) {
	return false, nil
}

func (f FakeTrigger) IAM() *ciam.Handle {
	return nil
}

// Verify that it satisfies the eventflow.Trigger interface.
var _ Trigger = &trigger{}

// Exists implements eventflow.Trigger.Exists
func (t *trigger) Exists(ctx context.Context) (bool, error) {
	return t.trigger.Exists(ctx)
}

// Delete implements eventflow.Trigger.Delete
func (t *trigger) Delete(ctx context.Context) error {
	return t.trigger.Delete(ctx)
}

func (t *trigger) IAM() iam.Handle {
	return iam.NewIamHandle(t.trigger.IAM())
}

func (t *trigger) ID() string {
	return t.trigger.ID()
}
