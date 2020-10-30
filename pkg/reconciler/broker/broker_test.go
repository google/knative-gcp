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

package broker

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/google/knative-gcp/pkg/broker/ingress"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/network"
	. "knative.dev/pkg/reconciler/testing"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/broker/resources"
	brokercellresources "github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
)

const (
	testNS     = "testnamespace"
	brokerName = "test-broker"

	testProject = "test-project-id"
	testUID     = "abc123"
	systemNS    = "knative-testing"

	brokerFinalizerName = "brokers.eventing.knative.dev"
)

var (
	backoffPolicy           = eventingduckv1beta1.BackoffPolicyLinear
	backoffDelay            = "PT5S"
	deadLetterTopicID       = "test-dead-letter-topic-id"
	retry             int32 = 3

	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	brokerFinalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`)
	brokerReconciledEvent       = Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "testnamespace/test-broker"`)
	brokerFinalizedEvent        = Eventf(corev1.EventTypeNormal, "BrokerFinalized", `Broker finalized: "testnamespace/test-broker"`)
	ingressServiceName          = brokercellresources.Name(resources.DefaultBrokerCellName, brokercellresources.IngressName)

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, systemNS, network.GetClusterDomainName()),
		Path:   ingress.BrokerPath(testNS, brokerName),
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

func TestAllCases(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  testKey,
	}, {
		Name: "Broker is being deleted, no topic or sub exists",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithInitBrokerConditions,
				WithBrokerDeletionTimestamp,
				WithBrokerSetDefaults,
			),
		},
		WantEvents: []string{
			brokerFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Broker is being deleted, topic and sub exists",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithInitBrokerConditions,
				WithBrokerDeletionTimestamp,
				WithBrokerSetDefaults,
			),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-bkr_testnamespace_test-broker_abc123", "cre-bkr_testnamespace_test-broker_abc123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Create broker with ready brokercell, broker is created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerReadyURI(brokerAddress),
				WithBrokerSetDefaults,
			),
		}},
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-bkr_testnamespace_test-broker_abc123"),
			SubscriptionExists("cre-bkr_testnamespace_test-broker_abc123"),
		},
	}, {
		Name: "Create broker with unready brokercell, broker is created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults,
			),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellIngressFailed("", ""),
				WithBrokerCellSetDefaults,
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerReadyURI(brokerAddress),
				WithBrokerBrokerCellUnknown("BrokerCellNotReady", "Brokercell knative-testing/default is not ready"),
				WithBrokerSetDefaults,
			),
		}},
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-bkr_testnamespace_test-broker_abc123"),
			SubscriptionExists("cre-bkr_testnamespace_test-broker_abc123"),
		},
	}, {
		Name: "Create broker without brokercell, brokercell creation failed",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults,
			),
		},
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "brokercells"),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{
			{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithBrokerUID(testUID),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithInitBrokerConditions,
					WithBrokerBrokerCellFailed("BrokerCellCreationFailed", "Failed to create knative-testing/default"),
					WithBrokerSetDefaults,
				),
			},
		},
		WantCreates:             []runtime.Object{resources.CreateBrokerCell(nil) /*Currently brokercell doesn't require broker information*/},
		SkipNamespaceValidation: true, // The brokercell resource is created in a different namespace (system namespace) than the broker
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to reconcile broker: brokercell reconcile failed: inducing failure for create brokercells`),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		WantErr: true,
	}, {
		Name: "Create broker without brokercell, both broker and brokercell are created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults,
			),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{
			{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(brokerv1beta1.BrokerClass),
					WithBrokerUID(testUID),
					WithBrokerDeliverySpec(brokerDeliverySpec),
					WithBrokerReadyURI(brokerAddress),
					WithBrokerBrokerCellUnknown("BrokerCellNotReady", "Brokercell knative-testing/default is not ready"),
					WithBrokerSetDefaults,
				),
			},
		},
		WantCreates:             []runtime.Object{resources.CreateBrokerCell(nil) /*Currently brokercell doesn't require broker information*/},
		SkipNamespaceValidation: true, // The brokercell resource is created in a different namespace (system namespace) than the broker
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "BrokerCellCreated", `Created brokercell knative-testing/default`),
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig("cre-bkr_testnamespace_test-broker_abc123", &pubsub.TopicConfig{
				Labels: map[string]string{
					"broker_class": "googlecloud", "name": "test-broker", "namespace": "testnamespace", "resource": "brokers",
				},
			}),
			SubscriptionExists("cre-bkr_testnamespace_test-broker_abc123"),
		},
	}, {
		Name: "Check topic config with correct data residency and label",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerReadyURI(brokerAddress),
				WithBrokerSetDefaults,
			),
		}},
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre":                    []PubsubAction{},
			"dataResidencyConfigMap": NewDataresidencyConfigMapFromRegions([]string{"us-east1"}),
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExistsWithConfig("cre-bkr_testnamespace_test-broker_abc123", &pubsub.TopicConfig{
				MessageStoragePolicy: pubsub.MessageStoragePolicy{
					AllowedPersistenceRegions: []string{"us-east1"},
				},
				Labels: map[string]string{
					"broker_class": "googlecloud", "name": "test-broker", "namespace": "testnamespace", "resource": "brokers",
				},
			}),
		},
	}, {
		Name: "Create broker with ready brokercell with nil Pubsub client",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerReadyURI(brokerAddress),
				WithBrokerSetDefaults,
			),
		}},
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeNormal, "TopicCreated", `Created PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionCreated", `Created PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerReconciledEvent,
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{},
			// TODO: This should make sure the function is called only once, but currently this case only check the create function
			// will be called on demand since there is no test that reconcile twice or with both reconcile and delete.
			"maxPSClientCreateTime": 1,
		},
		PostConditions: []func(*testing.T, *TableRow){
			TopicExists("cre-bkr_testnamespace_test-broker_abc123"),
			SubscriptionExists("cre-bkr_testnamespace_test-broker_abc123"),
		},
	}, {
		Name: "Create broker with pubsub client creation failure",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerSetDefaults),
			NewBrokerCell(resources.DefaultBrokerCellName, systemNS,
				WithBrokerCellReady,
				WithBrokerCellSetDefaults),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerDeliverySpec(brokerDeliverySpec),
				WithBrokerReadyURI(brokerAddress),
				WithBrokerSetDefaults,
				WithBrokerTopicUnknown("FinalizeTopicPubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
				WithBrokerSubscriptionUnknown("FinalizeSubscriptionPubSubClientCreationFailed", "Failed to create Pub/Sub client: Invoke time 0 reaches the max invoke time 0"),
			),
		}},
		WantEvents: []string{
			brokerFinalizerUpdatedEvent,
			Eventf(corev1.EventTypeWarning, "InternalError", "failed to reconcile broker: decoupling topic reconcile failed: Invoke time 0 reaches the max invoke time 0"),
		},
		WantPatches: []clientgotesting.PatchActionImpl{
			patchFinalizers(testNS, brokerName, brokerFinalizerName),
		},
		OtherTestData: map[string]interface{}{
			"pre":                   []PubsubAction{},
			"maxPSClientCreateTime": 0,
		},
		PostConditions: []func(*testing.T, *TableRow){},
		WantErr:        true,
	}}

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
			createPubsubClientFn = GetFailedTestClientCreateFunc(srv.Addr, maxTime.(int))
			testPSClient = nil
		}

		ctx = addressable.WithDuck(ctx)
		ctx = resource.WithDuck(ctx)
		r := &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			brokerCellLister:   listers.GetBrokerCellLister(),
			projectID:          testProject,
			pubsubClient:       testPSClient,
			dataresidencyStore: drStore,
		}
		return brokerreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetBrokerLister(), r.Recorder, r, brokerv1beta1.BrokerClass)
	}))
}

func patchFinalizers(namespace, name, finalizer string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizer + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
