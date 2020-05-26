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

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1alpha1/resource"
	brokerreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/broker/v1beta1/broker"
	"github.com/google/knative-gcp/pkg/reconciler"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS     = "testnamespace"
	brokerName = "test-broker"

	testProject = "test-project-id"
	generation  = 1
	testUID     = "abc123"
	systemNS    = "knative-testing"

	brokerFinalizerName = "brokers.eventing.knative.dev"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerName)

	brokerFinalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`)
	brokerReconciledEvent       = Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "testnamespace/test-broker"`)
	brokerFinalizedEvent        = Eventf(corev1.EventTypeNormal, "BrokerFinalized", `Broker finalized: "testnamespace/test-broker"`)

	brokerAddress = &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, systemNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/%s/%s", testNS, brokerName),
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
				WithBrokerDeletionTimestamp),
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
				WithBrokerDeletionTimestamp),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "TopicDeleted", `Deleted PubSub topic "cre-bkr_testnamespace_test-broker_abc123"`),
			Eventf(corev1.EventTypeNormal, "SubscriptionDeleted", `Deleted PubSub subscription "cre-bkr_testnamespace_test-broker_abc123"`),
			brokerFinalizedEvent,
		},
		OtherTestData: map[string]interface{}{
			"pre": []PubsubAction{
				TopicAndSub("cre-bkr_testnamespace_test-broker_abc123"),
			},
		},
		PostConditions: []func(*testing.T, *TableRow){
			NoTopicsExist(),
			NoSubscriptionsExist(),
		},
	}, {
		Name: "Broker created",
		Key:  testKey,
		Objects: []runtime.Object{
			NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID)),
			NewEndpoints(ingressServiceName, systemNS,
				WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: NewBroker(brokerName, testNS,
				WithBrokerClass(brokerv1beta1.BrokerClass),
				WithBrokerUID(testUID),
				WithBrokerReadyURI(brokerAddress),
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
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		// Insert pubsub client for PostConditions and create fixtures
		psclient, close := TestPubsubClient(ctx, testProject)
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

		ctx = addressable.WithDuck(ctx)
		ctx = resource.WithDuck(ctx)
		r := &Reconciler{
			Base:               reconciler.NewBase(ctx, controllerAgentName, cmw),
			triggerLister:      listers.GetTriggerLister(),
			configMapLister:    listers.GetConfigMapLister(),
			endpointsLister:    listers.GetEndpointsLister(),
			deploymentLister:   listers.GetDeploymentLister(),
			targetsConfig:      memory.NewEmptyTargets(),
			targetsNeedsUpdate: make(chan struct{}),
			projectID:          testProject,
			pubsubClient:       psclient,
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

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
