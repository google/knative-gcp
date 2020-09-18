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
	"net/url"
	"testing"

	"knative.dev/eventing/pkg/apis/eventing"
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
PubSubWithBrokerTestImpl tests the following scenario:

                              5                   4
                    ------------------   --------------------
                    |                 | |                    |
          1         v	      2       | v         3          |
(Sender or Source) ---> Broker(PubSub) ---> trigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func BrokerWithPubSubChannelTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)
	brokerURL, brokerName := createBrokerWithPubSubChannel(client)
	kngcphelpers.BrokerEventTransformationTestHelper(client, brokerURL, brokerName, false)
}

func PubSubSourceBrokerWithPubSubChannelTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	brokerURL, brokerName := createBrokerWithPubSubChannel(client)
	kngcphelpers.BrokerEventTransformationTestWithPubSubSourceHelper(client, authConfig, brokerURL, brokerName)
	// TODO(nlopezgi): assert StackDriver metrics after https://github.com/google/knative-gcp/issues/317 is resolved
}

func StorageSourceBrokerWithPubSubChannelTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	brokerURL, brokerName := createBrokerWithPubSubChannel(client)
	kngcphelpers.BrokerEventTransformationTestWithStorageSourceHelper(client, authConfig, brokerURL, brokerName)
}

func AuditLogsSourceBrokerWithPubSubChannelTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	brokerURL, brokerName := createBrokerWithPubSubChannel(client)
	kngcphelpers.BrokerEventTransformationTestWithAuditLogsSourceHelper(client, authConfig, brokerURL, brokerName)

}

func SchedulerSourceBrokerWithPubSubChannelTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	brokerURL, brokerName := createBrokerWithPubSubChannel(client)
	kngcphelpers.BrokerEventTransformationTestWithSchedulerSourceHelper(client, authConfig, brokerURL, brokerName)

}

func createBrokerWithPubSubChannel(client *lib.Client) (url.URL, string) {
	brokerName := helpers.AppendRandomString("pubsub")
	// Create a new Broker.
	// TODO(chizhg): maybe we don't need to create these RBAC resources as they will now be automatically created?
	client.Core.CreateRBACResourcesForBrokers()
	client.Core.CreateBrokerConfigMapOrFail(brokerName, lib.ChannelTypeMeta)
	client.Core.CreateBrokerV1Beta1OrFail(brokerName,
		eventingtestresources.WithBrokerClassForBrokerV1Beta1(eventing.MTChannelBrokerClassValue),
		eventingtestresources.WithConfigMapForBrokerConfig(),
	)

	// Wait for broker ready.
	client.Core.WaitForResourceReadyOrFail(brokerName, eventingtestlib.BrokerTypeMeta)

	// Get broker URL.
	metaAddressable := eventingtestresources.NewMetaResource(brokerName, client.Namespace, eventingtestlib.BrokerTypeMeta)
	u, err := duck.GetAddressableURI(client.Core.Dynamic, metaAddressable)
	if err != nil {
		client.T.Error(err.Error())
	}
	return u, brokerName
}
