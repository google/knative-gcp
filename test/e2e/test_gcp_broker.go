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

	"knative.dev/eventing/test/lib/duck"

	v1 "k8s.io/api/core/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingtestlib "knative.dev/eventing/test/lib"
	eventingtestresources "knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
)

/*
PubSubWithBrokerTestImpl tests the following scenario:

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
	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")

	client := lib.Setup(t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(client)

	// Create a target Job to receive the events.
	makeTargetJobOrDie(client, targetName)

	u := createGCPBroker(t, client, targetName)

	// Just to make sure all resources are ready.
	time.Sleep(10 * time.Second)

	// Create a sender Job to sender the event.
	senderJob := resources.SenderJob(senderName, []v1.EnvVar{{
		Name:  "BROKER_URL",
		Value: u.String(),
	}})
	client.CreateJobOrFail(senderJob)

	// Check if dummy CloudEvent is sent out.
	if done := jobDone(client, senderName, t); !done {
		t.Error("dummy event wasn't sent to broker")
		t.Failed()
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName, t); !done {
		t.Error("resp event didn't hit the target pod")
		t.Failed()
	}
}

func createGCPBroker(t *testing.T, client *lib.Client, targetName string) url.URL {
	brokerName := helpers.AppendRandomString("gcp")
	dummyTriggerName := "dummy-broker-" + brokerName
	respTriggerName := "resp-broker-" + brokerName
	kserviceName := helpers.AppendRandomString("kservice")

	// Create a new GCP Broker.
	client.Core.CreateBrokerV1Beta1OrFail(brokerName, eventingtestresources.WithBrokerClassForBrokerV1Beta1("googlecloud"))

	// Create the Knative Service.
	kservice := resources.ReceiverKService(
		kserviceName, client.Namespace)
	client.CreateUnstructuredObjOrFail(kservice)

	// Create a Trigger with the Knative Service subscriber.
	client.Core.CreateTriggerOrFail(
		dummyTriggerName,
		eventingtestresources.WithBroker(brokerName),
		eventingtestresources.WithAttributesTriggerFilter(
			eventingv1alpha1.TriggerAnyFilter, eventingv1alpha1.TriggerAnyFilter,
			map[string]interface{}{"type": "e2e-testing-dummy"}),
		eventingtestresources.WithSubscriberServiceRefForTrigger(kserviceName),
	)

	// Create a Trigger with the target Service subscriber.
	client.Core.CreateTriggerOrFail(
		respTriggerName,
		eventingtestresources.WithBroker(brokerName),
		eventingtestresources.WithAttributesTriggerFilter(
			eventingv1alpha1.TriggerAnyFilter, eventingv1alpha1.TriggerAnyFilter,
			map[string]interface{}{"type": "e2e-testing-resp"}),
		eventingtestresources.WithSubscriberServiceRefForTrigger(targetName),
	)

	// Wait for broker, trigger, ksvc ready.
	client.Core.WaitForResourceReadyOrFail(brokerName, eventingtestlib.BrokerTypeMeta)
	client.Core.WaitForResourcesReadyOrFail(eventingtestlib.TriggerTypeMeta)
	client.Core.WaitForResourceReadyOrFail(kserviceName, lib.KsvcTypeMeta)

	// Get broker URL.
	metaAddressable := eventingtestresources.NewMetaResource(brokerName, client.Namespace, eventingtestlib.BrokerTypeMeta)
	u, err := duck.GetAddressableURI(client.Core.Dynamic, metaAddressable)
	if err != nil {
		t.Error(err.Error())
	}
	return u
}
