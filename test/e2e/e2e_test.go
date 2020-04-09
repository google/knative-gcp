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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conformancehelpers "knative.dev/eventing/test/conformance/helpers"
	e2ehelpers "knative.dev/eventing/test/e2e/helpers"
	eventingtestlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/logstream"

	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	"github.com/google/knative-gcp/test/e2e/lib"
)

// All e2e tests go below:

// TestSmoke makes sure we can run tests.
func TestSmokeChannel(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeTestChannelImpl(t, authConfig)
}

func TestSingleBinaryEventForChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/486 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.SingleEventForChannelTestHelper(t, cloudevents.Binary, "v1alpha1", channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestSingleStructuredEventForChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/486 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.SingleEventForChannelTestHelper(t, cloudevents.Structured, "v1alpha1", channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestChannelClusterDefaulter(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/knative/eventing-contrib/issues/627 is fixed")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelClusterDefaulterTestHelper(t, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestChannelNamespaceDefaulter(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/knative/eventing-contrib/issues/627 is fixed")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelNamespaceDefaulterTestHelper(t, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestEventTransformationForSubscription(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.EventTransformationForSubscriptionTestHelper(t, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestChannelChain(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelChainTestHelper(t, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestEventTransformationForTrigger(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.EventTransformationForTriggerTestHelper(t, "ChannelBasedBroker" /*brokerClass*/, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestBrokerChannelFlow(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.BrokerChannelFlowTestHelper(t, "ChannelBasedBroker" /*brokerClass*/, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestChannelDeadLetterSink(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/485 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelDeadLetterSinkTestHelper(t, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestBrokerDeadLetterSink(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/485 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.BrokerDeadLetterSinkTestHelper(t, "ChannelBasedBroker" /*brokerClass*/, channelTestRunner, lib.DuplicatePubSubSecret)
}

func TestBrokerTracing(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	conformancehelpers.BrokerTracingTestHelperWithChannelTestRunner(
		t, "ChannelBasedBroker", channelTestRunner,
		func(client *eventingtestlib.Client) {
			lib.GetCredential(client, authConfig.WorkloadIdentity)
			lib.SetTracingToZipkin(client)
		},
	)
}

func TestChannelTracing(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	conformancehelpers.ChannelTracingTestHelper(t, metav1.TypeMeta{
		APIVersion: messagingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}, func(client *eventingtestlib.Client) {
		// This test is running based on code in knative/eventing, so it does not use the same
		// Client that tests in this repo use. Therefore, we need to duplicate the logic from this
		// repo's Setup() here. See test/e2e/lifecycle.go's Setup() for the function used in this
		// repo whose functionality we need to copy here.

		// Copy the secret from the default namespace to the namespace used in the test.
		lib.GetCredential(client, authConfig.WorkloadIdentity)
		lib.SetTracingToZipkin(client)
	})
}

// TestSmokePullSubscription makes sure we can run tests on PullSubscriptions.
func TestSmokePullSubscription(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePullSubscriptionTestImpl(t, authConfig)
}

// TestPullSubscriptionWithTarget tests we can knock down a target.
func TestPullSubscriptionWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PullSubscriptionWithTargetTestImpl(t, authConfig)
}

// TestSmokeCloudPubSubSource makes sure we can run tests on the CloudPubSubSource.
func TestSmokeCloudPubSubSource(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudPubSubSourceTestImpl(t, authConfig)
}

// TestCloudPubSubSourceWithTarget tests we can knock down a target from a CloudPubSubSource.
func TestCloudPubSubSourceWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudPubSubSourceWithTargetTestImpl(t, false /*assertMetrics */, authConfig)
}

// TestCloudPubSubSourceStackDriverMetrics tests we can knock down a target from a CloudPubSubSource and that we send metrics to StackDriver.
func TestCloudPubSubSourceStackDriverMetrics(t *testing.T) {
	t.Skip("See issues https://github.com/google/knative-gcp/issues/317 and https://github.com/cloudevents/sdk-go/pull/234")
	cancel := logstream.Start(t)
	defer cancel()
	CloudPubSubSourceWithTargetTestImpl(t, true /*assertMetrics */, authConfig)
}

// TestBrokerWithPubSubChannel tests we can knock a Knative Service from a broker with PubSub Channel.
func TestBrokerWithPubSubChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	BrokerWithPubSubChannelTestImpl(t, authConfig)
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

// TestCloudStorageSourceBrokerWithPubSubChannel tests we can knock a Knative Service from a broker with PubSub Channel from a CloudStorageSource.
func TestCloudStorageSourceBrokerWithPubSubChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	StorageSourceBrokerWithPubSubChannelTestImpl(t, authConfig)
}

// TestCloudAuditLogsSourceBrokerWithPubSubChannel tests we can knock a Knative Service from a broker with PubSub Channel from a CloudAuditLogsSource.
func TestCloudAuditLogsSourceBrokerWithPubSubChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	AuditLogsSourceBrokerWithPubSubChannelTestImpl(t, authConfig)
}

// TestCloudSchedulerSourceBrokerWithPubSubChannel tests we can knock a Knative Service from a broker with PubSub Channel from a CloudSchedulerSource.
func TestCloudSchedulerSourceBrokerWithPubSubChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	SchedulerSourceBrokerWithPubSubChannelTestImpl(t, authConfig)
}

// TestCloudStorageSource tests we can knock down a target from a CloudStorageSource.
func TestCloudStorageSource(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudStorageSourceWithTestImpl(t, false /*assertMetrics */, authConfig)
}

// TestCloudStorageSourceStackDriverMetrics tests we can knock down a target from a CloudStorageSource and that we send metrics to StackDriver.
func TestCloudStorageSourceStackDriverMetrics(t *testing.T) {
	t.Skip("See issue https://github.com/google/knative-gcp/issues/317")
	cancel := logstream.Start(t)
	defer cancel()
	CloudStorageSourceWithTestImpl(t, true /*assertMetrics */, authConfig)
}

// TestCloudAuditLogsSource tests we can knock down a target from an CloudAuditLogsSource.
func TestCloudAuditLogsSource(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudAuditLogsSourceWithTestImpl(t, authConfig)
}

// TestSmokeCloudSchedulerSourceSetup tests if we can create a CloudSchedulerSource resource and get it to a ready state.
func TestSmokeCloudSchedulerSourceSetup(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudSchedulerSourceSetup(t, authConfig)
}

// TestCloudSchedulerSourceWithTargetTestImpl tests if we can receive an event on a bespoke sink from a CloudSchedulerSource source.
func TestCloudSchedulerSourceWithTargetTestImpl(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudSchedulerSourceWithTargetTestImpl(t, authConfig)
}
