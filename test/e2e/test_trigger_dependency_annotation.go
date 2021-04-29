// +build e2e

/*
Copyright 2020 The Knative Authors
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
	"encoding/base64"
	"testing"

	"cloud.google.com/go/pubsub"
	v1 "github.com/google/knative-gcp/pkg/schemas/v1"
	"github.com/google/knative-gcp/test/lib"

	. "github.com/cloudevents/sdk-go/v2/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	eventingresources "knative.dev/eventing/test/lib/resources"
)

// This test is for avoiding regressions on the trigger dependency annotation functionality.
// It will first create a trigger with the dependency annotation, and then create a CloudPubSubSource.
// Broker controller should make trigger become ready after CloudPubSubSource is ready.
func TriggerDependencyAnnotationTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	const (
		triggerName          = "trigger-annotation"
		subscriberName       = "subscriber-annotation"
		dependencyAnnotation = `{"kind":"CloudPubSubSource","name":"test-pubsub-source-annotation","apiVersion":"events.cloud.google.com/v1"}`
		pubSubSourceName     = "test-pubsub-source-annotation"
	)
	ctx := context.Background()

	projectName := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()

	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)
	_, brokerName := createGCPBroker(client)

	// Create subscribers.
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client.Core, subscriberName)

	// Create triggers.
	client.Core.CreateTriggerV1OrFail(triggerName,
		eventingresources.WithBrokerV1(brokerName),
		eventingresources.WithSubscriberServiceRefForTriggerV1(subscriberName),
		eventingresources.WithDependencyAnnotationTrigger(dependencyAnnotation),
	)

	pubSubConfig := lib.PubSubConfig{
		PubSubName:         pubSubSourceName,
		TopicName:          topicName,
		ServiceAccountName: authConfig.ServiceAccountName,
		SinkGVK:            lib.BrokerGVK,
		SinkName:           brokerName,
	}
	lib.MakePubSubOrDie(client, pubSubConfig)

	// Trigger should become ready after CloudPubSubSource was created
	client.Core.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)

	// publish a message to PubSub
	topic := lib.GetTopic(t, topicName)
	message := "Hello World"
	r := topic.Publish(context.TODO(), &pubsub.Message{
		Data: []byte(message),
	})
	_, err := r.Get(context.TODO())
	if err != nil {
		t.Logf("%s", err)
	}

	// verify that the event is delivered
	eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
		HasSource(v1.CloudPubSubEventSource(projectName, topicName)),
		DataContains(base64.StdEncoding.EncodeToString([]byte(message))),
	))
}
