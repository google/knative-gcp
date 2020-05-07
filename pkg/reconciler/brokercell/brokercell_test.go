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

package brokercell

import (
	"context"
	"fmt"
	"testing"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/testingdata"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS                  = "testnamespace"
	brokerCellName          = "test-brokercell"
	//brokerCellFinalizerName = "brokercells.internal.events.cloud.google.com"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerCellName)

	brokerCellReconciledEvent       = Eventf(corev1.EventTypeNormal, "BrokerCellReconciled", `BrokerCell reconciled: "testnamespace/test-brokercell"`)
	brokerCellUpdateFailedEvent     = Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-brokercell": inducing failure for update brokercells`)
	brokerCellFinalizedEvent        = Eventf(corev1.EventTypeNormal, "BrokerCellFinalized", `BrokerCell finalized: "testnamespace/test-brokercell"`)
	ingressDeploymentCreatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-ingress")
	ingressDeploymentUpdatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-ingress")
	fanoutDeploymentCreatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-fanout")
	fanoutDeploymentUpdatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-fanout")
	retryDeploymentCreatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-retry")
	retryDeploymentUpdatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-retry")
	ingressServiceCreatedEvent   = Eventf(corev1.EventTypeNormal, "ServiceCreated", "Created service testnamespace/test-brokercell-brokercell-ingress")
	ingressServiceUpdatedEvent   = Eventf(corev1.EventTypeNormal, "ServiceUpdated", "Updated service testnamespace/test-brokercell-brokercell-ingress")
	deploymentCreationFailedEvent   = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create deployments")
	deploymentUpdateFailedEvent     = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update deployments")
	serviceCreationFailedEvent      = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services")
	serviceUpdateFailedEvent        = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update services")
)

func init() {
	// Add types to scheme
	_ = intv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
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
			Name: "BrokerCell is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellDeletionTimestamp),
			},
		},
		{
			Name: "Ingress Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressFailed("IngressDeploymentFailed", `Failed to reconcile ingress deployment: inducing failure for create deployments`),
				),
			}},
			WantEvents: []string{
				deploymentCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.IngressDeployment(t),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				// Create an deployment such that only the spec is different from expected deployment to trigger an update.
				NewDeployment(brokerCellName+"-brokercell-ingress", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.IngressDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.IngressDeployment(t).ObjectMeta
					},
				),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressFailed("IngressDeploymentFailed", `Failed to reconcile ingress deployment: inducing failure for update deployments`),
				),
			}},
			WantEvents: []string{
				deploymentUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.IngressDeployment(t)},
			},
			WantErr: true,
		},
		{
			Name: "Ingress Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				testingdata.IngressDeploymentWithStatus(t),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressFailed("IngressServiceFailed", `Failed to reconcile ingress service: inducing failure for create services`),
				),
			}},
			WantEvents: []string{
				serviceCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.IngressService(t),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Service.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				testingdata.IngressDeploymentWithStatus(t),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				NewService(brokerCellName+"-brokercell-ingress", testNS,
					func(s *corev1.Service) {
						s.TypeMeta = testingdata.IngressService(t).TypeMeta
						s.ObjectMeta = testingdata.IngressService(t).ObjectMeta
					}),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressFailed("IngressServiceFailed", `Failed to reconcile ingress service: inducing failure for update services`),
				),
			}},
			WantEvents: []string{
				serviceUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.IngressService(t)},
			},
			WantErr: true,
		},
		{
			Name: "Fanout Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("FanoutDeploymentFailed", `Failed to reconcile fanout deployment: inducing failure for create deployments`),
				),
			}},
			WantEvents: []string{
				deploymentCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.FanoutDeployment(t),
			},
			WantErr: true,
		},
		{
			Name: "Fanout Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				// Create an deployment such that only the spec is different from expected deployment to trigger an update.
				NewDeployment(brokerCellName+"-brokercell-fanout", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.FanoutDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.FanoutDeployment(t).ObjectMeta
					},
				),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("FanoutDeploymentFailed", `Failed to reconcile fanout deployment: inducing failure for update deployments`),
				),
			}},
			WantEvents: []string{
				deploymentUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.FanoutDeployment(t)},
			},
			WantErr: true,
		},
		{
			Name: "Retry Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("RetryDeploymentFailed", `Failed to reconcile retry deployment: inducing failure for create deployments`),
				),
			}},
			WantEvents: []string{
				
				deploymentCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.RetryDeployment(t),
			},
			WantErr: true,
		},
		{
			Name: "Retry Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				// Create an deployment such that only the spec is different from expected deployment to trigger an update.
				NewDeployment(brokerCellName+"-brokercell-retry", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.RetryDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.RetryDeployment(t).ObjectMeta
					},
				),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("RetryDeploymentFailed", `Failed to reconcile retry deployment: inducing failure for update deployments`),
				),
			}},
			WantEvents: []string{
				deploymentUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.RetryDeployment(t)},
			},
			WantErr: true,
		},
		{
			Name: "BrokerCell created, resources created but resource status not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS),
			},
			WantCreates: []runtime.Object{
				testingdata.IngressDeployment(t),
				testingdata.IngressService(t),
				testingdata.FanoutDeployment(t),
				testingdata.RetryDeployment(t),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					// optimistically set everything to be ready, the following options will override individual conditions
					WithBrokerCellReady,
					// For newly created deployments and services, there statues are not ready because
					// we don't have a controller in the tests to mark their statues ready.
					// We only verify that they are created in the WantCreates.
					WithBrokerCellIngressFailed("EndpointsUnavailable", `Endpoints "test-brokercell-brokercell-ingress" is unavailable.`),
					WithBrokerCellFanoutFailed("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-fanout" is unavailable.`),
					WithBrokerCellRetryFailed("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-retry" is unavailable.`),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
				)},
			},
			WantEvents: []string{
				ingressDeploymentCreatedEvent,
				ingressServiceCreatedEvent,
				fanoutDeploymentCreatedEvent,
				retryDeploymentCreatedEvent,
				brokerCellReconciledEvent,
			},
		},
		{
			Name: "BrokerCell created successfully but status update failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "brokercells"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellReady,
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
				)},
			},
			WantEvents: []string{
				brokerCellUpdateFailedEvent,
			},
			WantErr: true,
		},
		{
			Name: "BrokerCell created successfully",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellReady,
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
				)},
			},
			WantEvents: []string{
				brokerCellReconciledEvent,
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		setReconcilerEnv()
		base := reconciler.NewBase(ctx, controllerAgentName, cmw)
		r, err := NewReconciler(base, listers.GetK8sServiceLister(), listers.GetEndpointsLister(), listers.GetDeploymentLister())
		if err != nil {
			t.Fatalf("Failed to created BrokerCell reconciler: %v", err)
		}
		return bcreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetBrokerCellLister(), r.Recorder, r)
	}))
}

