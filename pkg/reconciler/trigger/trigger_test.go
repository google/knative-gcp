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

package trigger

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	triggerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/trigger"
	"github.com/google/knative-gcp/pkg/reconciler"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS      = "testnamespace"
	triggerName = "test-trigger"
	brokerName  = "test-broker"
	testUID     = "abc123"
	testProject = "test-project-id"

	subscriberURI     = "http://example.com/subscriber/"
	subscriberKind    = "Service"
	subscriberName    = "subscriber-name"
	subscriberGroup   = "serving.knative.dev"
	subscriberVersion = "v1"
)

var (
	backoffPolicy           = eventingduckv1beta1.BackoffPolicyLinear
	backoffDelay            = "PT5S"
	deadLetterTopicID       = "test-dead-letter-topic-id"
	retry             int32 = 3

	testKey = fmt.Sprintf("%s/%s", testNS, triggerName)

	triggerFinalizerUpdatedEvent   = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-trigger" finalizers`)
	triggerReconciledEvent         = Eventf(corev1.EventTypeNormal, "TriggerReconciled", `Trigger reconciled: "testnamespace/test-trigger"`)
	triggerFinalizedEvent          = Eventf(corev1.EventTypeNormal, "TriggerFinalized", `Trigger finalized: "testnamespace/test-trigger"`)
	topicCreatedEvent              = Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-tgr_testnamespace_test-trigger_abc123"`)
	topicDeletedEvent              = Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-tgr_testnamespace_test-trigger_abc123"`)
	deadLetterTopicCreatedEvent    = Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "test-dead-letter-topic-id"`)
	subscriptionCreatedEvent       = Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-tgr_testnamespace_test-trigger_abc123"`)
	subscriptionDeletedEvent       = Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-tgr_testnamespace_test-trigger_abc123"`)
	subscriptionConfigUpdatedEvent = Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-tgr_testnamespace_test-trigger_abc123"`)
	subscriberAPIVersion           = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)
	subscriberGVK                  = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}
	brokerDeliverySpec = &eventingduckv1beta1.DeliverySpec{
		BackoffDelay:  &backoffDelay,
		BackoffPolicy: &backoffPolicy,
		Retry:         &retry,
		DeadLetterSink: &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "pubsub",
				Host:   deadLetterTopicID,
			},
		},
	}
)

func init() {
	// Add types to scheme
	_ = brokerv1beta1.AddToScheme(scheme.Scheme)
}

func TestAllCasesTrigger(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			Key:  "too/many/parts",
		},
		{
			Name: "key not found",
			Key:  testKey,
		},
		{
			Name: "Trigger with finalizer is being deleted, no topic or sub exists",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerDeletionTimestamp,
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults),
			},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				triggerFinalizedEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, triggerName),
			},
			OtherTestData: map[string]interface{}{},
			PostConditions: []func(*testing.T, *TableRow){
				NoTopicsExist(),
				NoSubscriptionsExist(),
			},
		},
		{
			// The finalization logic should be skipped since there is no finalizer string.
			Name: "Trigger without finalizer is being deleted, no topic or sub exists",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerDeletionTimestamp,
					WithTriggerSetDefaults),
			},
		},
		{
			Name: "Trigger is being deleted, topic and sub exists",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerDeletionTimestamp,
					WithTriggerUID(testUID),
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults),
			},
			WantEvents: []string{
				topicDeletedEvent,
				subscriptionDeletedEvent,
				triggerFinalizerUpdatedEvent,
				triggerFinalizedEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, triggerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					TopicAndSub("cre-tgr_testnamespace_test-trigger_abc123", "cre-tgr_testnamespace_test-trigger_abc123"),
				},
			},
			PostConditions: []func(*testing.T, *TableRow){
				NoTopicsExist(),
				NoSubscriptionsExist(),
			},
		},
		{
			Name: "Broker not found, Trigger with finalizer should be finalized",
			Key:  testKey,
			Objects: []runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults,
					WithInitTriggerConditions,
				),
			},
			WantEvents: []string{
				topicDeletedEvent,
				subscriptionDeletedEvent,
				triggerFinalizedEvent,
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					TopicAndSub("cre-tgr_testnamespace_test-trigger_abc123", "cre-tgr_testnamespace_test-trigger_abc123"),
				},
			},
		},
		{
			Name: "Broker is being deleted, Trigger with finalizer should be finalized",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerDeletionTimestamp,
					WithBrokerSetDefaults,
				),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults,
					WithInitTriggerConditions,
				),
			},
			WantEvents: []string{
				topicDeletedEvent,
				subscriptionDeletedEvent,
				triggerFinalizedEvent,
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					TopicAndSub("cre-tgr_testnamespace_test-trigger_abc123", "cre-tgr_testnamespace_test-trigger_abc123"),
				},
			},
		},
		{
			Name: "Switched to other brokerclass, Trigger with finalizer should be finalized",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass("some-other-brokerclass"),
					WithBrokerSetDefaults,
				),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults,
					WithInitTriggerConditions,
				),
			},
			WantEvents: []string{
				topicDeletedEvent,
				subscriptionDeletedEvent,
				triggerFinalizedEvent,
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					TopicAndSub("cre-tgr_testnamespace_test-trigger_abc123", "cre-tgr_testnamespace_test-trigger_abc123"),
				},
			},
			// This WantStatusUpdates should be deleted once the following TODO is finished.
			// TODO(https://github.com/knative/pkg/issues/1149) Add a FilterKind to genreconciler so it will
			// skip a trigger if it's not pointed to a gcp broker and doesn't have googlecloud finalizer string.
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerFinalizers(finalizerName),
					WithTriggerSetDefaults,
					WithInitTriggerConditions,
					WithTriggerTopicReady,
				),
			}},
		},
		{
			Name: "Broker is not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerUnknown("Broker/", ""),
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerDependencyReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				topicCreatedEvent,
				subscriptionCreatedEvent,
				triggerReconciledEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{},
			PostConditions: []func(*testing.T, *TableRow){
				OnlyTopics("cre-tgr_testnamespace_test-trigger_abc123"),
				OnlySubscriptions("cre-tgr_testnamespace_test-trigger_abc123"),
				SubscriptionHasRetryPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.RetryPolicy{
						MaximumBackoff: 5 * time.Second,
						MinimumBackoff: 5 * time.Second,
					}),
			},
		},
		{
			Name: "Subsciber doesn't exist",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerSetDefaults),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions,
					WithTriggerBrokerReady,
					WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `services.serving.knative.dev "subscriber-name" not found`),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", `services.serving.knative.dev "subscriber-name" not found`),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			WantErr: true,
		},
		{
			Name: "Trigger created, broker ready, subscriber is addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady,
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerDependencyReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				topicCreatedEvent,
				subscriptionCreatedEvent,
				triggerReconciledEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					Topic("test-dead-letter-topic-id"),
				},
			},
			PostConditions: []func(*testing.T, *TableRow){
				OnlyTopics("cre-tgr_testnamespace_test-trigger_abc123", "test-dead-letter-topic-id"),
				OnlySubscriptions("cre-tgr_testnamespace_test-trigger_abc123"),
				SubscriptionHasRetryPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.RetryPolicy{
						MaximumBackoff: 5 * time.Second,
						MinimumBackoff: 5 * time.Second,
					}),
				SubscriptionHasDeadLetterPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.DeadLetterPolicy{
						MaxDeliveryAttempts: 3,
						DeadLetterTopic:     "projects/test-project-id/topics/test-dead-letter-topic-id",
					}),
				TopicExistsWithConfig("cre-tgr_testnamespace_test-trigger_abc123", &pubsub.TopicConfig{
					Labels: map[string]string{
						"name": "test-trigger", "namespace": "testnamespace", "resource": "triggers",
					},
				}),
			},
		},
		{
			Name: "Sub already exists, update config",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady,
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerDependencyReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				subscriptionConfigUpdatedEvent,
				triggerReconciledEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					TopicAndSub("cre-tgr_testnamespace_test-trigger_abc123", "cre-tgr_testnamespace_test-trigger_abc123"),
					Topic("test-dead-letter-topic-id"),
				},
			},
			PostConditions: []func(*testing.T, *TableRow){
				OnlyTopics("cre-tgr_testnamespace_test-trigger_abc123", "test-dead-letter-topic-id"),
				OnlySubscriptions("cre-tgr_testnamespace_test-trigger_abc123"),
				SubscriptionHasRetryPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.RetryPolicy{
						MaximumBackoff: 5 * time.Second,
						MinimumBackoff: 5 * time.Second,
					}),
				SubscriptionHasDeadLetterPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.DeadLetterPolicy{
						MaxDeliveryAttempts: 3,
						DeadLetterTopic:     "projects/test-project-id/topics/test-dead-letter-topic-id",
					}),
			},
		},
		{
			Name: "Check topic config and labels",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady,
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerDependencyReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				topicCreatedEvent,
				subscriptionCreatedEvent,
				triggerReconciledEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					Topic("test-dead-letter-topic-id"),
				},
				"dataResidencyConfigMap": NewDataresidencyConfigMapFromRegions([]string{"us-east1"}),
			},
			PostConditions: []func(*testing.T, *TableRow){
				OnlyTopics("cre-tgr_testnamespace_test-trigger_abc123", "test-dead-letter-topic-id"),
				OnlySubscriptions("cre-tgr_testnamespace_test-trigger_abc123"),
				SubscriptionHasRetryPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.RetryPolicy{
						MaximumBackoff: 5 * time.Second,
						MinimumBackoff: 5 * time.Second,
					}),
				SubscriptionHasDeadLetterPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.DeadLetterPolicy{
						MaxDeliveryAttempts: 3,
						DeadLetterTopic:     "projects/test-project-id/topics/test-dead-letter-topic-id",
					}),
				TopicExistsWithConfig("cre-tgr_testnamespace_test-trigger_abc123", &pubsub.TopicConfig{
					MessageStoragePolicy: pubsub.MessageStoragePolicy{
						AllowedPersistenceRegions: []string{"us-east1"},
					},
					Labels: map[string]string{
						"name": "test-trigger", "namespace": "testnamespace", "resource": "triggers",
					},
				}),
			},
		},
		{
			Name: "Trigger created, broker ready, subscriber is addressable, nil pubsub client",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady,
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerDependencyReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				topicCreatedEvent,
				subscriptionCreatedEvent,
				triggerReconciledEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					Topic("test-dead-letter-topic-id"),
				},
				// TODO: This should make sure the function is called only once, but currently this case only check the create function
				// will be called on demand since there is no test that reconcile twice or with both reconcile and delete.
				"maxPSClientCreateTime": 1,
			},
			PostConditions: []func(*testing.T, *TableRow){
				OnlyTopics("cre-tgr_testnamespace_test-trigger_abc123", "test-dead-letter-topic-id"),
				OnlySubscriptions("cre-tgr_testnamespace_test-trigger_abc123"),
				SubscriptionHasRetryPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.RetryPolicy{
						MaximumBackoff: 5 * time.Second,
						MinimumBackoff: 5 * time.Second,
					}),
				SubscriptionHasDeadLetterPolicy("cre-tgr_testnamespace_test-trigger_abc123",
					&pubsub.DeadLetterPolicy{
						MaxDeliveryAttempts: 3,
						DeadLetterTopic:     "projects/test-project-id/topics/test-dead-letter-topic-id",
					}),
				TopicExistsWithConfig("cre-tgr_testnamespace_test-trigger_abc123", &pubsub.TopicConfig{
					Labels: map[string]string{
						"name": "test-trigger", "namespace": "testnamespace", "resource": "triggers",
					},
				}),
			},
		},
		{
			Name: "Trigger created, broker ready, subscriber is addressable, pubsub client creation fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithInitBrokerConditions,
					WithBrokerReady("url"),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerSetDefaults,
				),
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerSetDefaults),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(testUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithTriggerBrokerReady,
					WithTriggerSubscriptionReady,
					WithTriggerTopicReady,
					WithTriggerSubscriberResolvedSucceeded,
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSetDefaults,
					WithTriggerDependencyUnknown("", ""),
					WithTriggerTopicUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
					WithTriggerSubscriptionUnknown("PubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
				),
			}},
			WantEvents: []string{
				triggerFinalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "Invoke time 0 reaches the max invoke time 0"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, triggerName, finalizerName),
			},
			OtherTestData: map[string]interface{}{
				"pre": []PubsubAction{
					Topic("test-dead-letter-topic-id"),
				},
				"maxPSClientCreateTime": 0,
			},
			WantErr: true,
		},
	}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		srv := pstest.NewServer()
		// Insert pubsub client for PostConditions and create fixtures
		psclient, _ := GetTestClientCreateFunc(srv.Addr)(ctx, testProject)
		savedCreateFn := createPubsubClientFn
		close := func() {
			srv.Close()
			createPubsubClientFn = savedCreateFn
		}
		t.Cleanup(close)
		var drStore *dataresidency.Store
		if testData != nil {
			InjectPubsubClient(testData, psclient)
			if testData["pre"] != nil {
				fixtures := testData["pre"].([]PubsubAction)
				for _, f := range fixtures {
					f(ctx, t, psclient)
				}
			}

			// If we found "dataResidencyConfigMap" in OtherData, we create a store with the configmap
			if cm, ok := testData["dataResidencyConfigMap"]; ok {
				drStore = NewDataresidencyTestStore(t, cm.(*corev1.ConfigMap))
			}
		}

		// If maxPSClientCreateTime is in testData, no pubsub client is passed to reconciler, the reconciler
		// will create one in demand
		testPSClient := psclient
		if maxTime, ok := testData["maxPSClientCreateTime"]; ok {
			// Overwrite the createPubsubClientFn to one that failed when called more than maxTime times.
			// maxTime=0 is used to inject error
			createPubsubClientFn = GetFailedTestClientCreateFunc(srv.Addr, maxTime.(int))
			testPSClient = nil
		}

		ctx = addressable.WithDuck(ctx)
		ctx = resource.WithDuck(ctx)
		ctx = source.WithDuck(ctx)

		r := &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			brokerLister:       listers.GetBrokerLister(),
			sourceTracker:      duck.NewListableTracker(ctx, source.Get, func(types.NamespacedName) {}, 0),
			addressableTracker: duck.NewListableTracker(ctx, addressable.Get, func(types.NamespacedName) {}, 0),
			uriResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			projectID:          testProject,
			pubsubClient:       testPSClient,
			dataresidencyStore: drStore,
		}

		return triggerreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetTriggerLister(), r.Recorder, r, withAgentAndFinalizer(nil))
	}))
}

func makeSubscriberAddressableAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": subscriberURI,
				},
			},
		},
	}
}

// TODO Move to a util package so all reconciler tests can use.
func patchFinalizers(namespace, name, finalizer string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizer + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

// TODO Move to a util package so all reconciler tests can use.
func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
