/*
Copyright 2019 Google LLC.

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

// Package iam provides interfaces and wrappers around the google iam client.
package iam

import (
	"context"

	"cloud.google.com/go/iam"
)

// Client matches the interface exposed by iam.Handle
// see https://godoc.org/cloud.google.com/go/iam#Handle
type Handle interface {
	// Policy see https://godoc.org/cloud.google.com/go/iam#Handle.Policy
	Policy(ctx context.Context) (*iam.Policy, error)

	// SetPolicy see https://godoc.org/cloud.google.com/go/iam#Handle.SetPolicy
	SetPolicy(ctx context.Context, policy *iam.Policy) error
}
