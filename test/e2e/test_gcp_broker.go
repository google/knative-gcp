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

package e2e

import (
	"net/url"
	"testing"
	"time"

	"github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	brokerresources "github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	knativegcptestresources "github.com/google/knative-gcp/test/e2e/lib/resources"
	eventingtestlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	eventingtestresources "knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/google/knative-gcp/test/e2e/lib"
	kngcphelpers "github.com/google/knative-gcp/test/e2e/lib/helpers"
)

/*
GCPBrokerTestImpl tests the following scenario:

                              5                   4
                    ------------------   --------------------
                    |                 | |                    |
          1         v	      2       | v         3          |
(Sender or source) ---> GCP Broker ---> trigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func GCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)
	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTestHelper(client, brokerURL, brokerName, true)
}

func GCPBrokerMetricsTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	projectID := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)
	client.SetupStackDriverMetrics(t)
	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationMetricsTestHelper(client, projectID, brokerURL, brokerName)
}

func GCPBrokerTracingTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	projectID := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	err := client.Core.Kube.UpdateConfigMap("cloud-run-events", "config-tracing", map[string]string{
		"backend":                "stackdriver",
		"stackdriver-project-id": projectID,
	})
	if err != nil {
		client.T.Fatalf("Unable to set the config-tracing ConfigMap: %v", err)
	}

	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTracingTestHelper(client, projectID, brokerURL, brokerName)
}

func PubSubSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTestWithPubSubSourceHelper(client, authConfig, brokerURL, brokerName)
}

func StorageSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTestWithStorageSourceHelper(client, authConfig, brokerURL, brokerName)
}

func AuditLogsSourceBrokerWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTestWithAuditLogsSourceHelper(client, authConfig, brokerURL, brokerName)

}

func SchedulerSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(client)
	kngcphelpers.BrokerEventTransformationTestWithSchedulerSourceHelper(client, authConfig, brokerURL, brokerName)

}

// SmokeGCPBrokerTestImpl tests we can create a GCPBroker to ready state and we can delete a GCPBroker and its underlying resources.
func SmokeGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerName := helpers.AppendRandomString("gcp")
	// Create a new GCP Broker.
	gcpBroker := client.CreateGCPBrokerV1Beta1OrFail(brokerName, knativegcptestresources.WithBrokerClassForBrokerV1Beta1(v1beta1.BrokerClass))

	// Wait for broker ready.
	client.Core.WaitForResourceReadyOrFail(brokerName, eventingtestlib.BrokerTypeMeta)

	brokerresources.GenerateDecouplingTopicName(gcpBroker)

	topicID := brokerresources.GenerateDecouplingTopicName(gcpBroker)
	subID := brokerresources.GenerateDecouplingSubscriptionName(gcpBroker)

	createdTopicExists := lib.TopicExists(t, topicID)
	if !createdTopicExists {
		t.Errorf("Expected topic%q to exist", topicID)
	}

	createdSubExists := lib.SubscriptionExists(t, subID)
	if !createdSubExists {
		t.Errorf("Expected subscription %q to exist", subID)
	}

	client.DeleteGCPBrokerOrFail(brokerName)
	//Wait for 120 seconds for subscription to get deleted in gcp
	time.Sleep(knativegcptestresources.WaitDeletionTime)

	deletedTopicExists := lib.TopicExists(t, topicID)
	if deletedTopicExists {
		t.Errorf("Expected topic %q to get deleted", topicID)
	}
	deletedSubExists := lib.SubscriptionExists(t, subID)
	if deletedSubExists {
		t.Errorf("Expected subscription %q to get deleted", subID)
	}
	t.Logf("topic id is: %v /n, sub id is: %v /n", topicID, subID)
	t.Logf("createdSubExists id is: %t /n, deletedSubExists is: %t /n", createdSubExists, deletedSubExists)
	t.Logf("createdTopicExists id is: %t /n, deletedTopicExists is: %t /n", createdTopicExists, deletedTopicExists)
}

func createGCPBroker(client *lib.Client) (url.URL, string) {
	brokerName := helpers.AppendRandomString("gcp")
	// Create a new GCP Broker.
	client.CreateGCPBrokerV1Beta1OrFail(brokerName, knativegcptestresources.WithBrokerClassForBrokerV1Beta1(v1beta1.BrokerClass))

	// Wait for broker ready.
	client.Core.WaitForResourceReadyOrFail(brokerName, eventingtestlib.BrokerTypeMeta)

	// Get broker URL.
	metaAddressable := eventingtestresources.NewMetaResource(brokerName, client.Namespace, eventingtestlib.BrokerTypeMeta)
	u, err := duck.GetAddressableURI(client.Core.Dynamic, metaAddressable)
	if err != nil {
		client.T.Error(err.Error())
	}
	// Avoid propagation delay between the controller reconciles the broker config and
	// the config being pushed to the configmap volume in the ingress pod.
	time.Sleep(knativegcptestresources.WaitBrokercellTime)
	return u, brokerName
}
