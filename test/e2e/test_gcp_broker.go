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
(Sender) ---> GCP Broker ---> dummyTrigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func GCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(t, client)
	kngcphelpers.BrokerEventTransformationTestHelper(t, client, brokerURL, brokerName)
}

func PubSubSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(t, client)
	kngcphelpers.BrokerEventTransformationTestWithPubSubSourceHelper(t, client, authConfig, brokerURL, brokerName)
}

func StorageSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(t, client)
	kngcphelpers.BrokerEventTransformationTestWithStorageSourceHelper(t, client, authConfig, brokerURL, brokerName)
}

func AuditLogsSourceBrokerWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(t, client)
	kngcphelpers.BrokerEventTransformationTestWithAuditLogsSourceHelper(t, client, authConfig, brokerURL, brokerName)

}

func SchedulerSourceWithGCPBrokerTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	brokerURL, brokerName := createGCPBroker(t, client)
	kngcphelpers.BrokerEventTransformationTestWithSchedulerSourceHelper(t, client, authConfig, brokerURL, brokerName)

}

func createGCPBroker(t *testing.T, client *lib.Client) (url.URL, string) {
	brokerName := helpers.AppendRandomString("gcp")
	// Create a new GCP Broker.
	client.Core.CreateBrokerV1Beta1OrFail(brokerName, eventingtestresources.WithBrokerClassForBrokerV1Beta1("googlecloud"))

	// Wait for broker ready.
	client.Core.WaitForResourceReadyOrFail(brokerName, eventingtestlib.BrokerTypeMeta)

	// Get broker URL.
	metaAddressable := eventingtestresources.NewMetaResource(brokerName, client.Namespace, eventingtestlib.BrokerTypeMeta)
	u, err := duck.GetAddressableURI(client.Core.Dynamic, metaAddressable)
	if err != nil {
		t.Error(err.Error())
	}
	return u, brokerName
}
