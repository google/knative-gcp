/*
Copyright 2019 Google LLC

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channel

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"

	brokercellresources "github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"

	"cloud.google.com/go/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"

	"cloud.google.com/go/pubsub/pstest"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	"github.com/google/knative-gcp/pkg/reconciler/celltenant"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"

	channelreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/messaging/v1beta1/channel"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"

	inteventsv1beta1 "github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	"github.com/google/knative-gcp/pkg/reconciler"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	channelName = "test-channel"

	testNS   = "testnamespace"
	systemNS = "knative-testing"

	channelUID = channelName + "-abc-123"

	testProject       = "test-project-id"
	testClusterRegion = "us-east1"

	subscriptionUID        = subscriptionName + "-def-123"
	subscriptionName       = "testsubscription"
	subscriptionGeneration = 314

	channelFinalizerName = "channels.messaging.cloud.google.com"

	newSubscription1UID = "new-1-uid"
	newSubscription2UID = "new-2-uid"
	newSubscription3UID = "new-3-uid"

	updatedSubscription1UID = "updated-1-uid"
	updatedSubscription2UID = "updated-2-uid"
	updatedSubscription3UID = "updated-3-uid"

	unchangedSubscription1UID = "unchanged-1-uid"
	unchangedSubscription2UID = "unchanged-2-uid"
	unchangedSubscription3UID = "unchanged-3-uid"

	deletedSubscription1UID = "deleted-1-uid"
)

var (
	trueVal  = true
	falseVal = false

	testKey = fmt.Sprintf("%s/%s", testNS, channelName)

	channelURI, _ = apis.ParseURL(
		fmt.Sprintf("http://default-brokercell-ingress.%s.svc.cluster.local/channel/%s/%s", systemNS, testNS, channelName))

	testTopicID = fmt.Sprintf("cre-ch_%s_%s_%s", testNS, channelName, channelUID)

	sinkGVK = metav1.GroupVersionKind{
		Group:   "testing.cloud.google.com",
		Version: "v1beta1",
		Kind:    "Sink",
	}

	subscriberDNS = "subscriber.mynamespace.svc.cluster.local"
	subscriberURI = apis.HTTP(subscriberDNS)

	replyDNS = "reply.mynamespace.svc.cluster.local"
	replyURI = apis.HTTP(replyDNS)

	gServiceAccount = "test123@test123.iam.gserviceaccount.com"

	channelFinalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-channel" finalizers`)
	channelReconciledEvent       = Eventf(corev1.EventTypeNormal, "ChannelReconciled", `Channel reconciled: "testnamespace/test-channel"`)
	channelFinalizedEvent        = Eventf(corev1.EventTypeNormal, "ChannelFinalized", `Channel finalized: "testnamespace/test-channel"`)
	ingressServiceName           = brokercellresources.Name(resources.DefaultBrokerCellName, brokercellresources.IngressName)
)

func init() {
	// Add types to scheme
	_ = inteventsv1beta1.AddToScheme(scheme.Scheme)
}

func patchFinalizers(namespace, name, finalizer string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizer + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func TestAllCasesChannel(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  testKey,
	}, {
		Name: "Channel is being deleted, no topic or sub exists",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithInitChannelConditions,
				WithChannelDeletionTimestamp,
				WithChannelSetDefaults,
			),
		},
		WantEvents: []string{
			channelFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Channel is being deleted, topic and sub exist",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithInitChannelConditions,
				WithChannelDeletionTimestamp,
				WithChannelSetDefaults,
			),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-ch_testnamespace_test-channel_test-channel-abc-123", "cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Channel with subscriber is being deleted, topic and sub exist",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithInitChannelConditions,
				WithChannelReadyURI(channelURI),
				WithChannelDeletionTimestamp,
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                subscriptionUID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					}),
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithInitChannelConditions,
				WithChannelDeletionTimestamp,
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			channelFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-ch_testnamespace_test-channel_test-channel-abc-123", "cre-ch_testnamespace_test-channel_test-channel-abc-123"),
				TopicAndSub("cre-sub_testnamespace_test-channel_testsubscription-def-123", "cre-sub_testnamespace_test-channel_testsubscription-def-123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Create channel with ready brokerCell, channel is created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
		},
	}, {
		Name: "Create channel with unready brokerCell, channel is created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
			),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellIngressFailed("", ""),
				WithBrokerCellSetDefaults,
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelBrokerCellUnknown("BrokerCellNotReady", "BrokerCell knative-testing/default is not ready"),
				WithChannelSetDefaults,
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
		},
	}, {
		Name: "Create channel without brokerCell, brokerCell creation failed",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
			),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "brokerCells"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{
			{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithInitChannelConditions,
					WithChannelBrokerCellFailed("BrokerCellCreationFailed", "Failed to create BrokerCell knative-testing/default"),
					WithChannelSetDefaults,
				),
			},
		},
		WantCreates:             []runtime.Object{resources.CreateBrokerCell(nil) /*Currently brokerCell doesn't require channel information*/},
		SkipNamespaceValidation: true, // The brokerCell resource is created in a different namespace (system namespace) than the channel
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to reconcile Channel: brokercell reconcile failed: inducing failure for create brokercells`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		WantErr: true,
	}, {
		Name: "Create channel without brokerCell, both channel and brokerCell are created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{
			{
				Object: NewChannel(channelName, testNS,
					WithChannelUID(channelUID),
					WithChannelReadyURI(channelURI),
					WithChannelBrokerCellUnknown("BrokerCellNotReady", "BrokerCell knative-testing/default is not ready"),
					WithChannelSetDefaults,
				),
			},
		},
		WantCreates:             []runtime.Object{resources.CreateBrokerCell(nil) /*Currently brokerCell doesn't require channel information*/},
		SkipNamespaceValidation: true, // The brokerCell resource is created in a different namespace (system namespace) than the channel
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "BrokerCellCreated", `Created BrokerCell knative-testing/default`),
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig("cre-ch_testnamespace_test-channel_test-channel-abc-123", &pubsub.TopicConfig{
				Labels: map[string]string{
					"name": "test-channel", "namespace": "testnamespace", "resource": "channels",
				},
			}),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
		},
	}, {
		Name: "Check topic config with correct data residency and label",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre":                    []PubsubAction{},
			"dataResidencyConfigMap": NewDataresidencyConfigMapFromRegions([]string{"us-east1"}),
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig("cre-ch_testnamespace_test-channel_test-channel-abc-123", &pubsub.TopicConfig{
				MessageStoragePolicy: pubsub.MessageStoragePolicy{
					AllowedPersistenceRegions: []string{"us-east1"},
				},
				Labels: map[string]string{
					"name": "test-channel", "namespace": "testnamespace", "resource": "channels",
				},
			}),
		},
	}, {
		Name: "Create channel with ready brokerCell with nil Pubsub client",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
			// TODO: This should make sure the function is called only once, but currently this case only check the create function
			// will be called on demand since there is no test that reconcile twice or with both reconcile and delete.
			"maxPSClientCreateTime": 1,
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
		},
	}, {
		Name: "Create channel with pubsub client creation failure",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				// TopicID is the empty string because we are using Ready just as a shortcut, rather
				// than calling each method directly.
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
				WithChannelTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
				WithChannelSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile Channel: decoupling topic reconcile failed: Invoke time 0 reaches the max invoke time 0"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre":                   []PubsubAction{},
			"maxPSClientCreateTime": 0,
		},
		PostConditions: []func(*testing.T, *TableRow){},
		WantErr:        true,
	}, {
		Name: "Channel with Subscriber",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					})),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                subscriptionUID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					}),
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-ch_testnamespace_test-channel_test-channel-abc-123"`),
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			TopicExists("cre-sub_testnamespace_test-channel_testsubscription-def-123"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_testsubscription-def-123"),
		},
	}, {
		Name: "Channel updates existing Subscription",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                subscriptionUID,
						ObservedGeneration: subscriptionGeneration - 1,
						Ready:              "True",
					}),
			),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           subscriptionUID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					}),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                subscriptionUID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					}),
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-ch_testnamespace_test-channel_test-channel-abc-123", "cre-ch_testnamespace_test-channel_test-channel-abc-123"),
				TopicAndSub("cre-sub_testnamespace_test-channel_testsubscription-def-123", "cre-sub_testnamespace_test-channel_testsubscription-def-123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			TopicExists("cre-sub_testnamespace_test-channel_testsubscription-def-123"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_testsubscription-def-123"),
		},
	}, {
		Name: "Channel removes existing Subscription",
		Key:  testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                subscriptionUID,
						ObservedGeneration: subscriptionGeneration - 1,
						Ready:              "True",
					}),
			),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-sub_testnamespace_test-channel_testsubscription-def-123"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-ch_testnamespace_test-channel_test-channel-abc-123", "cre-ch_testnamespace_test-channel_test-channel-abc-123"),
				TopicAndSub("cre-sub_testnamespace_test-channel_testsubscription-def-123", "cre-sub_testnamespace_test-channel_testsubscription-def-123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			OnlyTopics("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			OnlySubscriptions("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
		},
	}, {
		Name: "Channel creates new Subscription, ignores unchanged Subscription, deletes old Subscription",
		// This tests a Channel with:
		// - 3 new subscriptions, they are in the spec but not the status.
		// - 3 unchanged subscriptions, they are in the spec and the status.
		// - 3 updated subscriptions, they are in the spec and the status, but the
		//   ObservedGeneration in the status is older than the Generation in the spec.
		// - 1 old subscription that is to be deleted, it is in the status, but not the spec.
		//    - This was supposed to be three, but the TableTests enforce strict event ordering,
		//      the reconciler deletes old Subscriptions in a random order (iterating through a
		//      map), and I didn't want to slow down the real world use case to enforce an order
		//      purely for a test.
		Key: testKey,
		Objects: []runtime.Object{
			NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
				),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription1UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription1UID,
						ObservedGeneration: subscriptionGeneration - 1,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                deletedSubscription1UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription2UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription2UID,
						ObservedGeneration: subscriptionGeneration - 2,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription3UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription3UID,
						ObservedGeneration: subscriptionGeneration - 1,
						Ready:              "True",
					},
				),
			),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewChannel(channelName, testNS,
				WithChannelUID(channelUID),
				WithChannelReadyURI(channelURI),
				WithChannelSetDefaults,
				WithChannelSubscribers(
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription1UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription2UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           newSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           unchangedSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
					duckv1beta1.SubscriberSpec{
						UID:           updatedSubscription3UID,
						Generation:    subscriptionGeneration,
						SubscriberURI: subscriberURI,
						ReplyURI:      replyURI,
					},
				),
				WithChannelSubscribersStatus(
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription1UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription1UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                newSubscription1UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription2UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription2UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                newSubscription2UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                unchangedSubscription3UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                updatedSubscription3UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
					duckv1beta1.SubscriberStatus{
						UID:                newSubscription3UID,
						ObservedGeneration: subscriptionGeneration,
						Ready:              "True",
					},
				),
			),
		}},
		WantEvents: []string{
			channelFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-sub_testnamespace_test-channel_new-1-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-sub_testnamespace_test-channel_new-1-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_unchanged-1-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_updated-1-uid"`),
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-sub_testnamespace_test-channel_new-2-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-sub_testnamespace_test-channel_new-2-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_unchanged-2-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_updated-2-uid"`),
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-sub_testnamespace_test-channel_new-3-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-sub_testnamespace_test-channel_new-3-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_unchanged-3-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionConfigUpdated", `Updated config for PubSub subscription "cre-sub_testnamespace_test-channel_updated-3-uid"`),
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-sub_testnamespace_test-channel_deleted-1-uid"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-sub_testnamespace_test-channel_deleted-1-uid"`),
			channelReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, channelName, channelFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-ch_testnamespace_test-channel_test-channel-abc-123", "cre-ch_testnamespace_test-channel_test-channel-abc-123"),
				TopicAndSub("cre-sub_testnamespace_test-channel_unchanged-1-uid", "cre-sub_testnamespace_test-channel_unchanged-1-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_updated-1-uid", "cre-sub_testnamespace_test-channel_updated-1-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_deleted-1-uid", "cre-sub_testnamespace_test-channel_deleted-1-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_unchanged-2-uid", "cre-sub_testnamespace_test-channel_unchanged-2-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_updated-2-uid", "cre-sub_testnamespace_test-channel_updated-2-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_unchanged-3-uid", "cre-sub_testnamespace_test-channel_unchanged-3-uid"),
				TopicAndSub("cre-sub_testnamespace_test-channel_updated-3-uid", "cre-sub_testnamespace_test-channel_updated-3-uid"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			SubscriptionExists("cre-ch_testnamespace_test-channel_test-channel-abc-123"),
			TopicExists("cre-sub_testnamespace_test-channel_new-1-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_new-1-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_unchanged-1-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_unchanged-1-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_updated-1-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_updated-1-uid"),
			TopicDoesNotExist("cre-sub_testnamespace_test-channel_deleted-1-uid"),
			SubscriptionDoesNotExist("cre-sub_testnamespace_test-channel_deleted-1-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_new-2-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_new-2-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_unchanged-2-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_unchanged-2-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_updated-2-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_updated-2-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_new-3-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_new-3-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_unchanged-3-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_unchanged-3-uid"),
			TopicExists("cre-sub_testnamespace_test-channel_updated-3-uid"),
			SubscriptionExists("cre-sub_testnamespace_test-channel_updated-3-uid"),
		},
		CmpOpts: []cmp.Option{
			cmp.Transformer("alphabetizeSubscriberStatus", func(orig duckv1beta1.SubscribableStatus) duckv1beta1.SubscribableStatus {
				// The order of subscriber status objects are not guaranteed, so alphabetize them to
				// guarantee a consistent order.
				sorted := duckv1beta1.SubscribableStatus{
					Subscribers: make([]duckv1beta1.SubscriberStatus, len(orig.Subscribers)),
				}
				copy(sorted.Subscribers, orig.Subscribers)
				sort.Slice(sorted.Subscribers, func(i, j int) bool {
					return sorted.Subscribers[i].UID < sorted.Subscribers[j].UID
				})
				return sorted
			}),
		},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		srv := pstest.NewServer()
		// Insert pubsub client for PostConditions and create fixtures
		psclient, _ := GetTestClientCreateFunc(srv.Addr)(ctx, testProject)
		savedCreateFn := celltenant.CreatePubsubClientFn
		t.Cleanup(func() {
			srv.Close()
			celltenant.CreatePubsubClientFn = savedCreateFn
		})
		if testData != nil {
			InjectPubsubClient(testData, psclient)
			if testData["pre"] != nil {
				fixtures := testData["pre"].([]PubsubAction)
				for _, f := range fixtures {
					f(ctx, t, psclient)
				}
			}
		}
		// If we found "dataResidencyConfigMap" in OtherData, we create a store with the configmap
		var drStore *dataresidency.Store
		if cm, ok := testData["dataResidencyConfigMap"]; ok {
			drStore = NewDataresidencyTestStore(t, cm.(*corev1.ConfigMap))
		}

		// If maxPSClientCreateTime is in testData, no pubsub client is passed to reconciler, the reconciler
		// will create one in demand
		testPSClient := psclient
		if maxTime, ok := testData["maxPSClientCreateTime"]; ok {
			// Overwrite the createPubsubClientFn to one that failed when called more than maxTime times.
			// maxTime=0 is used to inject error
			celltenant.CreatePubsubClientFn = GetFailedTestClientCreateFunc(srv.Addr, maxTime.(int))
			testPSClient = nil
		}

		ctx = addressable.WithDuck(ctx)
		ctx = resource.WithDuck(ctx)
		r := &Reconciler{
			Reconciler: celltenant.Reconciler{
				Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
				BrokerCellLister:   listers.GetBrokerCellLister(),
				ProjectID:          testProject,
				PubsubClient:       testPSClient,
				DataresidencyStore: drStore,
				ClusterRegion:      testClusterRegion,
			},
			targetReconciler: &celltenant.TargetReconciler{
				ProjectID:          testProject,
				PubsubClient:       testPSClient,
				DataresidencyStore: drStore,
				ClusterRegion:      testClusterRegion,
			},
		}
		return channelreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetChannelLister(), r.Recorder, r)
	}))
}
