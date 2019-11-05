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
	"encoding/json"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/base"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

/*
PubSubWithBrokerTestImpl tests the following scenario:

                          5                 4
                    ------------------   --------------------
                    |                 | |                    |
              1     v	    2         | v        3           |
(Sender) ---> Broker(PubSub) ---> dummyTrigger -------> Knative Service(Receiver)
                    |
                    |    6                   7
                    |-------> respTrigger -------> Service(Target)

Note: the number denotes the sequence of the event that flows in this test case.
*/

func BrokerWithPubSubChannelTestImpl(t *testing.T, packages map[string]string) {
	brokerName := helpers.AppendRandomString("pubsub")
	dummyTriggerName := "dummy-broker-" + brokerName
	respTriggerName := "resp-broker-" + brokerName
	kserviceName := helpers.AppendRandomString("kservice")
	senderName := helpers.AppendRandomString("sender")
	targetName := helpers.AppendRandomString("target")
	clusterRoleName := helpers.AppendRandomString("e2e-pubsub")

	client := Setup(t, true)
	defer TearDown(client)

	config := map[string]string{
		"namespace":        client.Namespace,
		"brokerName":       brokerName,
		"dummyTriggerName": dummyTriggerName,
		"respTriggerName":  respTriggerName,
		"kserviceName":     kserviceName,
		"senderName":       senderName,
		"targetName":       targetName,
		"clusterRoleName":  clusterRoleName,
	}
	for k, v := range packages {
		config[k] = v
	}

	// Create resources.
	brokerInstaller := createResource(client, config, []string{"pubsub_broker", "istio"}, t)
	defer deleteResource(brokerInstaller, t)

	// Wait for broker, trigger, ksvc ready.
	brokerGVR := schema.GroupVersionResource{
		Group:    "eventing.knative.dev",
		Version:  "v1alpha1",
		Resource: "brokers",
	}
	if err := client.WaitForResourceReady(client.Namespace, brokerName, brokerGVR); err != nil {
		t.Error(err)
	}

	triggerGVR := schema.GroupVersionResource{
		Group:    "eventing.knative.dev",
		Version:  "v1alpha1",
		Resource: "triggers",
	}

	if err := client.WaitForResourceReady(client.Namespace, dummyTriggerName, triggerGVR); err != nil {
		t.Error(err)
	}
	if err := client.WaitForResourceReady(client.Namespace, respTriggerName, triggerGVR); err != nil {
		t.Error(err)
	}

	ksvcGVR := schema.GroupVersionResource{
		Group:    "serving.knative.dev",
		Version:  "v1",
		Resource: "services",
	}
	if err := client.WaitForResourceReady(client.Namespace, kserviceName, ksvcGVR); err != nil {
		t.Error(err)
	}

	// Get broker URL.
	metaAddressable := resources.NewMetaResource(brokerName, client.Namespace, common.BrokerTypeMeta)
	u, err := base.GetAddressableURI(client.Dynamic, metaAddressable)
	if err != nil {
		t.Error(err.Error())
	}
	config["brokerURL"] = u.String()

	// Just to make sure all resources are ready.
	time.Sleep(5 * time.Second)

	// Send a dummy CloudEvent to broker.
	senderInstaller := createResource(client, config, []string{"pubsub_sender"}, t)
	defer deleteResource(senderInstaller, t)

	jobGVR := schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}
	// Check if dummy CloudEvent is sent out.
	if done := jobDone(client, senderName, t, jobGVR); !done {
		t.Error("dummy event wasn't sent to broker")
		t.Failed()
	}
	// Check if resp CloudEvent hits the target Service.
	if done := jobDone(client, targetName, t, jobGVR); !done {
		t.Error("resp event didn't hit the target pod")
		t.Failed()
	}
}

func createResource(client *Client, config map[string]string, folders []string, t *testing.T) *Installer {
	installer := NewInstaller(client.Dynamic, config,
		EndToEndConfigYaml(folders)...)
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return nil
	}
	return installer
}

func deleteResource(installer *Installer, t *testing.T) {
	if err := installer.Do("delete"); err != nil {
		t.Errorf("failed to delete, %s", err)
	}
	// Wait for resources to be deleted.
	time.Sleep(15 * time.Second)
}

func jobDone(client *Client, podName string, t *testing.T, jobGVR schema.GroupVersionResource) bool {
	msg, err := client.WaitUntilJobDone(client.Namespace, podName)
	if err != nil {
		t.Error(err)
		return false
	}
	if msg == "" {
		t.Error("No terminating message from the pod")
		return false
	} else {
		out := &TargetOutput{}
		if err := json.Unmarshal([]byte(msg), out); err != nil {
			t.Error(err)
			return false
		}
		if !out.Success {
			if logs, err := client.LogsFor(client.Namespace, podName, jobGVR); err != nil {
				t.Error(err)
			} else {
				t.Logf("job: %s\n", logs)
			}
			return false
		}
	}
	return true
}
