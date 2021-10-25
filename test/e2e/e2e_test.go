//go:build e2e
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

	conformancehelpers "knative.dev/eventing/test/conformance/helpers"
	e2ehelpers "knative.dev/eventing/test/e2e/helpers"
	eventingtestlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/logstream"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/google/knative-gcp/test/lib"
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
	e2ehelpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		binding.EncodingBinary, e2ehelpers.SubscriptionV1,
		"",
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

func TestSingleStructuredEventForChannel(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/486 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.SingleEventForChannelTestHelper(
		context.Background(),
		t,
		binding.EncodingStructured,
		e2ehelpers.SubscriptionV1,
		"",
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

func TestChannelClusterDefaulter(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/knative/eventing-contrib/issues/627 is fixed")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelClusterDefaulterTestHelper(
		context.Background(),
		t,
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

func TestChannelNamespaceDefaulter(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/knative/eventing-contrib/issues/627 is fixed")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelNamespaceDefaulterTestHelper(
		context.Background(),
		t,
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

func TestEventTransformationForSubscription(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.EventTransformationForSubscriptionTestHelper(
		context.Background(),
		t,
		e2ehelpers.SubscriptionV1,
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

func TestChannelChain(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelChainTestHelper(
		context.Background(),
		t,
		e2ehelpers.SubscriptionV1,
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

func TestChannelDeadLetterSink(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/485 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	e2ehelpers.ChannelDeadLetterSinkTestHelper(
		context.Background(),
		t,
		e2ehelpers.SubscriptionV1,
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

func TestChannelTracing(t *testing.T) {
	if authConfig.WorkloadIdentity {
		t.Skip("Skip broker related test when workloadIdentity is enabled, issue: https://github.com/google/knative-gcp/issues/746")
	}
	t.Skip("Skipping until https://github.com/google/knative-gcp/issues/1455 is fixed.")
	cancel := logstream.Start(t)
	defer cancel()
	conformancehelpers.ChannelTracingTestHelperWithChannelTestRunner(context.Background(), t,
		channelTestRunner,
		func(client *eventingtestlib.Client) {
			// This test is running based on code in knative/eventing, so it does not use the same
			// Client that tests in this repo use. Therefore, we need to duplicate the logic from this
			// repo's Setup() here. See test/e2e/lifecycle.go's Setup() for the function used in this
			// repo whose functionality we need to copy here.

			// Copy the secret from the default namespace to the namespace used in the test.
			lib.GetCredential(context.Background(), client, authConfig.WorkloadIdentity)
			lib.SetTracingToZipkin(context.Background(), client)
		})
}

// TestSmokePullSubscriptionV1 test we can create a v1 PullSubscription to ready state
// We keep a set of smoke tests for each supported version of PullSubscription to make sure the webhook works.
func TestSmokePullSubscriptionV1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePullSubscriptionTestHelper(t, authConfig, "v1")
}

// TestSmokePullSubscriptionV1beta1 test we can create a v1beta1 PullSubscription to ready state
// We keep a set of smoke tests for each supported version of PullSubscription to make sure the webhook works.
func TestSmokePullSubscriptionV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePullSubscriptionTestHelper(t, authConfig, "v1beta1")
}

// TestPullSubscriptionWithTarget tests we can knock down a target.
func TestPullSubscriptionWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PullSubscriptionWithTargetTestImpl(t, authConfig)
}

// TestSmokeCloudPubSubSourceWithDeletion test we can create a CloudPubSubSource to ready state and we can delete a CloudPubSubSource and its underlying resources.
func TestSmokeCloudPubSubSourceWithDeletion(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudPubSubSourceWithDeletionTestImpl(t, authConfig)
}

// TestSmokeCloudPubSubSourceV1beta1 we can create a v1beta1 CloudPubSubSource to ready state.
// We keep a set of smoke tests for each supported version of CloudPubSubSource to make sure the webhook works.
func TestSmokeCloudPubSubSourceV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudPubSubSourceTestHelper(t, authConfig, "v1beta1")
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

// TestSmokeCloudBuildSourceV1beta1 we can create a v1beta1 CloudBuildSource to ready state.
// We keep a set of smoke tests for each supported version of CloudBuildSource to make sure the webhook works.
func TestSmokeCloudBuildSourceV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudBuildSourceTestHelper(t, authConfig, "v1beta1")
}

// TestSmokeCloudBuildSourceWithDeletion we can create a CloudBuildSource to ready state and we can delete a CloudBuildSource and its underlying resources.
func TestSmokeCloudBuildSourceWithDeletion(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudBuildSourceWithDeletionTestImpl(t, authConfig)
}

// TestCloudBuildSourceWithTarget tests we can knock down a target from a CloudBuildSource.
func TestCloudBuildSourceWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudBuildSourceWithTargetTestImpl(t, authConfig)
}

// TestSmokeCloudStorageSourceWithDeletion tests if we can create a CloudStorageSource to ready state and delete a CloudStorageSource and its underlying resources.
func TestSmokeCloudStorageSourceWithDeletion(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudStorageSourceWithDeletionTestImpl(t, authConfig)
}

// TestCloudStorageSourceWithTarget tests we can knock down a target from a CloudStorageSource.
func TestCloudStorageSourceWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudStorageSourceWithTargetTestImpl(t, false /*assertMetrics */, authConfig)
}

// TestCloudStorageSourceStackDriverMetrics tests we can knock down a target from a CloudStorageSource and that we send metrics to StackDriver.
func TestCloudStorageSourceStackDriverMetrics(t *testing.T) {
	t.Skip("See issue https://github.com/google/knative-gcp/issues/317")
	cancel := logstream.Start(t)
	defer cancel()
	CloudStorageSourceWithTargetTestImpl(t, true /*assertMetrics */, authConfig)
}

// TestSmokeCloudPubSubSourceV1beta1 we can create a v1beta1 CloudAuditLogsSource to ready state.
// We keep a set of smoke tests for each supported version of CloudAuditLogsSource to make sure the webhook works.
func TestSmokeCloudAuditLogsSourceV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudAuditLogsSourceTestHelper(t, authConfig, "v1beta1")
}

// TestSmokeCloudAuditLogsSourceWithDeletion tests if we can create a CloudAuditLogsSource to ready state and delete a CloudAuditLogsSource and its underlying resources.
func TestSmokeCloudAuditLogsSourceWithDeletion(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudAuditLogsSourceWithDeletionTestImpl(t, authConfig)
}

// TestCloudAuditLogsSource tests we can knock down a target from an CloudAuditLogsSource.
func TestCloudAuditLogsSourceWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudAuditLogsSourceWithTargetTestImpl(t, authConfig)
}

// TestSmokeCloudSchedulerSourceV1beta1 we can create a v1beta1 CloudSchedulerSource to ready state.
// We keep a set of smoke tests for each supported version of CloudSchedulerSource to make sure the webhook works.
func TestSmokeCloudSchedulerSourceV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudSchedulerSourceTestHelper(t, authConfig, "v1beta1")
}

// TestSmokeCloudStorageSourceV1beta1 we can create a v1beta1 CloudStorageSource to ready state.
// We keep a set of smoke tests for each supported version of CloudStorageSource to make sure the webhook works.
func TestSmokeCloudStorageSourceV1beta1(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudStorageSourceTestHelper(t, authConfig, "v1beta1")
}

// TestSmokeCloudSchedulerSourceWithDeletion tests if we can create a CloudSchedulerSource to ready state and delete a CloudSchedulerSource and its underlying resources.
func TestSmokeCloudSchedulerSourceWithDeletion(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeCloudSchedulerSourceWithDeletionTestImpl(t, authConfig)
}

// TestCloudSchedulerSourceWithTargetTestImpl tests if we can receive an event on a bespoke sink from a CloudSchedulerSource source.
func TestCloudSchedulerSourceWithTargetTestImpl(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudSchedulerSourceWithTargetTestImpl(t, authConfig)
}

// TestSmokeGCPBroker tests if we can create a GCPBroker to ready state and delete a GCPBroker and its underlying resources.
func TestSmokeGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeGCPBrokerTestImpl(t, authConfig)
}

// TestGCPBroker tests we can knock a Knative Service from a gcp broker.
func TestGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	GCPBrokerTestImpl(t, authConfig)
}

// TestGCPBrokerMetrics tests we can knock a Knative Service from a GCP broker and the GCP Broker correctly reports its metrics to StackDriver.
func TestGCPBrokerMetrics(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	GCPBrokerMetricsTestImpl(t, authConfig)
}

// TestGCPBroker tests we can knock a Knative Service from a gcp broker.
func TestGCPBrokerTracing(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	GCPBrokerTracingTestImpl(t, authConfig)
}

// TestCloudPubSubSourceWithGCPBroker tests we can knock a Knative Service from a GCPBroker from a CloudPubSubSource.
func TestCloudPubSubSourceWithGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PubSubSourceWithGCPBrokerTestImpl(t, authConfig)
}

// TestCloudStorageSourceWithGCPBroker tests we can knock a Knative Service from a GCPBroker from a CloudStorageSource.
func TestCloudStorageSourceWithGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	StorageSourceWithGCPBrokerTestImpl(t, authConfig)
}

// TestCloudAuditLogsSourceWithGCPBroker tests we can knock a Knative Service from a GCPBroker from a CloudAuditLogsSource.
func TestCloudAuditLogsSourceWithGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	AuditLogsSourceBrokerWithGCPBrokerTestImpl(t, authConfig)
}

// TestCloudSchedulerSourceWithGCPBroker tests we can knock a Knative Service from a GCPBroker from a CloudSchedulerSource.
func TestCloudSchedulerSourceWithGCPBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SchedulerSourceWithGCPBrokerTestImpl(t, authConfig)
}

// TestTriggerDependencyAnnotation tests that Trigger with DependencyAnnotation works.
func TestTriggerDependencyAnnotation(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	TriggerDependencyAnnotationTestImpl(t, authConfig)
}

// TestCloudLoggingGCPControlPlane tests that log messages written by the knative-gcp Controller and
// Webhook are written to the GCP Cloud Logging API.
func TestCloudLoggingGCPControlPlane(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudLoggingGCPControlPlaneTestImpl(t, authConfig)
}

// TestCloudLoggingGCPSource tests that log messages written by the CloudPubSubSource, as an exemplar
// of all knative-gcp sources, are written to the GCP Cloud Logging API.
func TestCloudLoggingGCPSource(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudLoggingCloudPubSubSourceTestImpl(t, authConfig)
}

// TestCloudLoggingGCPTopic tests that log messages written by the Topic data plane publisher are
// written to the GCP Cloud Logging API.
func TestCloudLoggingGCPTopic(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	CloudLoggingTopicTestImpl(t, authConfig)
}

// TestAuthCheckForPodCheck tests the authentication check functionality which is running inside of the Pod.
func TestAuthCheckForPodCheck(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	AuthCheckForPodCheckTestImpl(t, authConfig)
}

// TestAuthCheckForNonPodCheck tests the authentication check functionality which is running outside of the Pod.
// e.g. checking if k8s service account or k8s secret are correctly set.
func TestAuthCheckForNonPodCheck(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	AuthCheckForNonPodCheckTestImpl(t, authConfig)
}
