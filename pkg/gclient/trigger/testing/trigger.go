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

package testing

import (
	"context"

	"github.com/google/knative-gcp/pkg/gclient/iam"
	testiam "github.com/google/knative-gcp/pkg/gclient/iam/testing"
	gtrigger "github.com/google/knative-gcp/pkg/gclient/trigger"
)

// TestTrigger is a test Pub/Sub trigger.
type testTrigger struct {
	data       TestTriggerData
	handleData testiam.TestHandleData
	id         string
}

// TestTriggerData is the data used to configure the test Trigger.
type TestTriggerData struct {
	ExistsErr error
	Exists    bool
	DeleteErr error
}

// Verify that it satisfies the pubsub.Trigger interface.
var _ gtrigger.Trigger = &testTrigger{}

// Exists implements Trigger.Exists.
func (t *testTrigger) Exists(ctx context.Context) (bool, error) {
	return t.data.Exists, t.data.ExistsErr
}

func (t *testTrigger) Delete(ctx context.Context) error {
	return t.data.DeleteErr
}

func (t *testTrigger) IAM() iam.Handle {
	return testiam.NewTestHandle(t.handleData)
}

func (t *testTrigger) ID() string {
	return t.id
}
