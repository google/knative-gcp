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

package e2e

import (
	"context"
	"testing"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	kngcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	"github.com/google/knative-gcp/test/lib"
)

// SmokeTestChannelImpl makes sure we can run tests.
func SmokeTestChannelImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	t.Helper()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	channel := kngcptesting.NewChannel("e2e-smoke-test", client.Namespace, kngcptesting.WithChannelServiceAccount(authConfig.ServiceAccountName))
	client.CreateChannelOrFail(channel)

	client.Core.WaitForResourceReadyOrFail("e2e-smoke-test", lib.ChannelTypeMeta)
}
