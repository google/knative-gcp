// +build e2e

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

package conformance

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	e2ehelpers "knative.dev/eventing/test/e2e/helpers"
	eventingtestlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/logstream"

	"github.com/google/knative-gcp/test/lib"
)

// All conformance tests go below:

func TestEventTransformationForTrigger(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()

	brokerClass := "MTChannelBasedBroker"
	brokerVersion := "v1beta1"
	triggerVersion := "v1beta1"
	channelTestRunner.RunTests(t, eventingtestlib.FeatureBasic, func(t *testing.T, component metav1.TypeMeta) {
		e2ehelpers.EventTransformationForTriggerTestHelper(
			context.Background(),
			t,
			brokerVersion,
			triggerVersion,
			e2ehelpers.ChannelBasedBrokerCreator(component, brokerClass),
			func(client *eventingtestlib.Client) {
				// This test is running based on code in knative/eventing, so it does not use the same
				// Client that tests in this repo use. Therefore, we need to duplicate the logic from this
				// repo's Setup() here. See test/e2e/lifecycle.go's Setup() for the function used in this
				// repo whose functionality we need to copy here.

				// Copy the secret from the default namespace to the namespace used in the test.
				lib.GetCredential(context.Background(), client, authConfig.WorkloadIdentity)
			},
		)
	})
}

func TestBrokerChannelFlow(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()

	brokerClass := "MTChannelBasedBroker"
	brokerVersion := "v1beta1"
	triggerVersion := "v1beta1"
	e2ehelpers.BrokerChannelFlowWithTransformation(
		context.Background(),
		t,
		brokerClass,
		brokerVersion,
		triggerVersion,
		channelTestRunner,
		func(client *eventingtestlib.Client) {
			// This test is running based on code in knative/eventing, so it does not use the same
			// Client that tests in this repo use. Therefore, we need to duplicate the logic from this
			// repo's Setup() here. See test/e2e/lifecycle.go's Setup() for the function used in this
			// repo whose functionality we need to copy here.

			// Copy the secret from the default namespace to the namespace used in the test.
			lib.GetCredential(context.Background(), client, authConfig.WorkloadIdentity)
		},
	)
}

// TestCloudPubSubSourceBrokerWithPubSubChannel tests we can knock a Knative Service from a broker with PubSub Channel from a CloudPubSubSource.
func TestCloudPubSubSourceBrokerWithPubSubChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	PubSubSourceBrokerWithPubSubChannelTestImpl(t, authConfig)
}
