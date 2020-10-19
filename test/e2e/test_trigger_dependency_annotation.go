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
	"fmt"
	"testing"

	"github.com/google/knative-gcp/test/lib"

	. "github.com/cloudevents/sdk-go/v2/test"
	"k8s.io/apimachinery/pkg/util/uuid"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	gcptesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	eventingresources "knative.dev/eventing/test/lib/resources"
)

// This test is for avoiding regressions on the trigger dependency annotation functionality.
// It will first create a trigger with the dependency annotation, and then create a pingSource.
// Broker controller should make trigger become ready after pingSource is ready.
func TriggerDependencyAnnotationTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	const (
		triggerName          = "trigger-annotation"
		subscriberName       = "subscriber-annotation"
		dependencyAnnotation = `{"kind":"PingSource","name":"test-ping-source-annotation","apiVersion":"sources.knative.dev/v1beta1"}`
		pingSourceName       = "test-ping-source-annotation"
		// Every 1 minute starting from now
		schedule = "*/1 * * * *"
	)
	ctx := context.Background()
	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)
	_, brokerName := createGCPBroker(client)

	// Create subscribers.
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client.Core, subscriberName)

	// Create triggers.
	client.Core.CreateTriggerOrFailV1Beta1(triggerName,
		eventingresources.WithBrokerV1Beta1(brokerName),
		eventingresources.WithSubscriberServiceRefForTriggerV1Beta1(subscriberName),
		eventingresources.WithDependencyAnnotationTriggerV1Beta1(dependencyAnnotation),
	)

	jsonData := fmt.Sprintf(`{"msg":"Test trigger-annotation %s"}`, uuid.NewUUID())
	pingSource := eventingtesting.NewPingSourceV1Beta1(
		pingSourceName,
		client.Namespace,
		eventingtesting.WithPingSourceV1B1Spec(sourcesv1beta1.PingSourceSpec{
			Schedule: schedule,
			JsonData: jsonData,
			SourceSpec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						APIVersion: gcptesting.ApiVersion(lib.BrokerGVK),
						Kind:       lib.BrokerGVK.Kind,
						Name:       brokerName,
					},
				},
			},
		}),
	)

	client.Core.CreatePingSourceV1Beta1OrFail(pingSource)

	// Trigger should become ready after pingSource was created
	client.Core.WaitForResourceReadyOrFail(triggerName, testlib.TriggerTypeMeta)

	eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
		HasSource(sourcesv1beta1.PingSourceSource(client.Namespace, pingSourceName)),
		HasData([]byte(jsonData)),
	))
}
