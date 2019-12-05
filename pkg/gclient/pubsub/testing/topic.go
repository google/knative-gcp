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

	"github.com/google/knative-gcp/pkg/gclient/pubsub"
)

// TestTopic is a test Pub/Sub topic.
type TestTopic struct {
	id string
}

// Verify that it satisfies the pubsub.Topic interface.
var _ pubsub.Topic = &TestTopic{}

// Exists implements Topic.Exists.
func (t *TestTopic) Exists(ctx context.Context) (bool, error) {
	return true, nil
}

func (t *TestTopic) Delete(ctx context.Context) error {
	return nil
}
