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

	//"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/gclient/iam"
)

// Client will matches the interface exposed by EventFlow triggers Client
type Client interface {
	// Assumed API
	Close() error
	// Assumed API
	Trigger(id string) Trigger
	// Assumed API
	CreateTrigger(ctx context.Context, id string, sourceType string, filters map[string]string) (Trigger, error)
}

type Trigger interface {
	Exists(ctx context.Context) (bool, error)

	Delete(ctx context.Context) error

	IAM() iam.Handle

	ID() string
}
