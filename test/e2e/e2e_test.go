// +build e2e

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

	e2ehelpers "knative.dev/eventing/test/e2e/helpers"
	"knative.dev/pkg/test/logstream"
)

// All e2e tests go below:

func TestEventTransformationForSubscription(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.EventTransformationForSubscriptionTestHelper(context.TODO(), t, e2ehelpers.SubscriptionV1beta1, channelTestRunner)
}
