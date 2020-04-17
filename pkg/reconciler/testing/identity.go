/*
Copyright 2020 Google LLC.

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

	"github.com/google/knative-gcp/pkg/reconciler/identity/iam"
)

var NoopIAMPolicyManager iam.IAMPolicyManager = noopManager(struct{}{})

type noopManager struct{}

func (noopManager) AddIAMPolicyBinding(ctx context.Context, account iam.GServiceAccount, member string, role iam.RoleName) error {
	return nil
}

func (noopManager) RemoveIAMPolicyBinding(ctx context.Context, account iam.GServiceAccount, member string, role iam.RoleName) error {
	return nil
}
