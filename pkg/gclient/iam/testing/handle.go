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

package testing

import (
	"context"

	"cloud.google.com/go/iam"
	giam "github.com/google/knative-gcp/pkg/gclient/iam"
)

type TestHandleData struct {
	PolicyErr    error
	SetPolicyErr error
}

type testHandle struct {
	Config TestHandleData

	policy iam.Policy
}

func (h *testHandle) Policy(ctx context.Context) (*iam.Policy, error) {
	if h.Config.PolicyErr != nil {
		return nil, h.Config.PolicyErr
	}
	return &h.policy, nil
}

func (h *testHandle) SetPolicy(ctx context.Context, policy *iam.Policy) error {
	if h.Config.SetPolicyErr != nil {
		h.policy = *policy
	}
	return h.Config.SetPolicyErr
}

func NewTestHandle(config TestHandleData) giam.Handle {
	return &testHandle{Config: config}
}
