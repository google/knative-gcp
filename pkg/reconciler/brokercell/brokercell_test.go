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

	appsv1 "k8s.io/api/apps/v1"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	. "knative.dev/pkg/reconciler/testing"

	"github.com/google/go-cmp/cmp"
	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	"github.com/google/knative-gcp/pkg/broker/config"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/testingdata"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"

	"google.golang.org/protobuf/proto"
)

const (
	testNS         = "testnamespace"
	brokerCellName = "test-brokercell"
	targetsCMName  = "broker-targets"
	targetsCMKey   = "targets"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerCellName)

	creatorAnnotation = map[string]string{"internal.events.cloud.google.com/creator": "googlecloud"}

	brokerCellReconciledEvent     = Eventf(corev1.EventTypeNormal, "BrokerCellReconciled", `BrokerCell reconciled: "testnamespace/test-brokercell"`)
	brokerCellGCEvent             = Eventf(corev1.EventTypeNormal, "BrokerCellGarbageCollected", `BrokerCell garbage collected: "testnamespace/test-brokercell"`)
	brokerCellGCFailedEvent       = Eventf(corev1.EventTypeWarning, "InternalError", `failed to garbage collect brokercell: inducing failure for delete brokercells`)
	brokerCellUpdateFailedEvent   = Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-brokercell": inducing failure for update brokercells`)
	ingressDeploymentCreatedEvent = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-ingress")
	ingressDeploymentUpdatedEvent = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-ingress")
	ingressHPACreatedEvent        = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerCreated", "Created HPA testnamespace/test-brokercell-brokercell-ingress-hpa")
	ingressHPAUpdatedEvent        = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerUpdated", "Updated HPA testnamespace/test-brokercell-brokercell-ingress-hpa")
	fanoutDeploymentCreatedEvent  = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-fanout")
	fanoutDeploymentUpdatedEvent  = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-fanout")
	fanoutHPACreatedEvent         = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerCreated", "Created HPA testnamespace/test-brokercell-brokercell-fanout-hpa")
	fanoutHPAUpdatedEvent         = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerUpdated", "Updated HPA testnamespace/test-brokercell-brokercell-fanout-hpa")
	retryDeploymentCreatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentCreated", "Created deployment testnamespace/test-brokercell-brokercell-retry")
	retryDeploymentUpdatedEvent   = Eventf(corev1.EventTypeNormal, "DeploymentUpdated", "Updated deployment testnamespace/test-brokercell-brokercell-retry")
	retryHPACreatedEvent          = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerCreated", "Created HPA testnamespace/test-brokercell-brokercell-retry-hpa")
	retryHPAUpdatedEvent          = Eventf(corev1.EventTypeNormal, "HorizontalPodAutoscalerUpdated", "Updated HPA testnamespace/test-brokercell-brokercell-retry-hpa")
	ingressServiceCreatedEvent    = Eventf(corev1.EventTypeNormal, "ServiceCreated", "Created service testnamespace/test-brokercell-brokercell-ingress")
	ingressServiceUpdatedEvent    = Eventf(corev1.EventTypeNormal, "ServiceUpdated", "Updated service testnamespace/test-brokercell-brokercell-ingress")
	deploymentCreationFailedEvent = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create deployments")
	deploymentUpdateFailedEvent   = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update deployments")
	serviceCreationFailedEvent    = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services")
	serviceUpdateFailedEvent      = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update services")
	hpaCreationFailedEvent        = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create horizontalpodautoscalers")
	hpaUpdateFailedEvent          = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update horizontalpodautoscalers")
	configmapCreationFailedEvent  = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create configmaps")
	configmapUpdateFailedEvent    = Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update configmaps")
	configmapCreatedEvent         = Eventf(corev1.EventTypeNormal, "ConfigMapCreated", "Created configmap testnamespace/test-brokercell-brokercell-broker-targets")
	configmapUpdatedEvent         = Eventf(corev1.EventTypeNormal, "ConfigMapUpdated", "Updated configmap testnamespace/test-brokercell-brokercell-broker-targets")
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
					WithBrokerCellDeletionTimestamp,
					WithBrokerCellSetDefaults,
				),
			},
		},
		{
			Name: "ConfigMap.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
			},
			WithReactors: []clientgotesting.ReactionFunc{InduceFailure("create", "configmaps")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigFailed(configFailed, "failed to update configmap: inducing failure for create configmaps"),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents:  []string{configmapCreationFailedEvent},
			WantCreates: []runtime.Object{testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults))},
			WantErr:     true,
		},
		{
			Name: "ConfigMap.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewBroker("broker", testNS, WithBrokerSetDefaults),
			},
			WithReactors: []clientgotesting.ReactionFunc{InduceFailure("update", "configmaps")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigFailed(configFailed, "failed to update configmap: inducing failure for update configmaps"),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{configmapUpdateFailedEvent},
			WantUpdates: []clientgotesting.UpdateActionImpl{{Object: testingdata.Config(t,
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				NewBroker("broker", testNS, WithBrokerSetDefaults))}},
			WantErr: true,
		},
		{
			Name: "Ingress Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("IngressDeploymentFailed", `Failed to reconcile ingress deployment: inducing failure for create deployments`),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				// Create an deployment such that only the spec is different from expected deployment to trigger an update.
				NewDeployment(brokerCellName+"-brokercell-ingress", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.IngressDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.IngressDeployment(t).ObjectMeta
					},
				),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("IngressDeploymentFailed", `Failed to reconcile ingress deployment: inducing failure for update deployments`),
					WithBrokerCellSetDefaults,
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
			Name: "Ingress HorizontalPodAutoscaler.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressDeploymentWithStatus(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile ingress HorizontalPodAutoscaler: inducing failure for create horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.IngressHPA(t),
			},
			WantErr: true,
		},
		{
			Name: "Ingress HorizontalPodAutoscaler.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressDeploymentWithStatus(t),
				emptyHPASpec(testingdata.IngressHPA(t)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile ingress HorizontalPodAutoscaler: inducing failure for update horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.IngressHPA(t)},
			},
			WantErr: true,
		},
		{
			Name: "Ingress Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("IngressServiceFailed", `Failed to reconcile ingress service: inducing failure for create services`),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
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
					WithTargetsCofigReady(),
					WithBrokerCellIngressFailed("IngressServiceFailed", `Failed to reconcile ingress service: inducing failure for update services`),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressHPA(t),
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
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("FanoutDeploymentFailed", `Failed to reconcile fanout deployment: inducing failure for create deployments`),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressHPA(t),
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
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("FanoutDeploymentFailed", `Failed to reconcile fanout deployment: inducing failure for update deployments`),
					WithBrokerCellSetDefaults,
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
			Name: "Fanout HorizontalPodAutoscaler.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressHPA(t),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeployment(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile fanout HorizontalPodAutoscaler: inducing failure for create horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.FanoutHPA(t),
			},
			WantErr: true,
		},
		{
			Name: "Fanout HorizontalPodAutoscaler.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressHPA(t),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeployment(t),
				emptyHPASpec(testingdata.FanoutHPA(t)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile fanout HorizontalPodAutoscaler: inducing failure for update horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.FanoutHPA(t)},
			},
			WantErr: true,
		},
		{
			Name: "Retry Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("RetryDeploymentFailed", `Failed to reconcile retry deployment: inducing failure for create deployments`),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
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
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("RetryDeploymentFailed", `Failed to reconcile retry deployment: inducing failure for update deployments`),
					WithBrokerCellSetDefaults,
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
			Name: "Retry HorizontalPodAutoscaler.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile retry HorizontalPodAutoscaler: inducing failure for create horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaCreationFailedEvent,
			},
			WantCreates: []runtime.Object{
				testingdata.RetryHPA(t),
			},
			WantErr: true,
		},
		{
			Name: "Retry HorizontalPodAutoscaler.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
				emptyHPASpec(testingdata.RetryHPA(t)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "horizontalpodautoscalers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBrokerCell(brokerCellName, testNS,
					WithInitBrokerCellConditions,
					WithTargetsCofigReady(),
					WithBrokerCellIngressAvailable(),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellFanoutAvailable(),
					WithBrokerCellRetryFailed("HorizontalPodAutoscalerFailed", `Failed to reconcile retry HorizontalPodAutoscaler: inducing failure for update horizontalpodautoscalers`),
					WithBrokerCellSetDefaults,
				),
			}},
			WantEvents: []string{
				hpaUpdateFailedEvent,
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.RetryHPA(t)},
			},
			WantErr: true,
		},
		{
			Name: "BrokerCell created, resources created but some resource status not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS),
			},
			WantCreates: []runtime.Object{
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				testingdata.IngressDeployment(t),
				testingdata.IngressHPA(t),
				testingdata.IngressService(t),
				testingdata.FanoutDeployment(t),
				testingdata.FanoutHPA(t),
				testingdata.RetryDeployment(t),
				testingdata.RetryHPA(t),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					// optimistically set everything to be ready, the following options will override individual conditions
					WithBrokerCellReady,
					// For newly created deployments and services, there statues are not ready because
					// we don't have a controller in the tests to mark their statues ready.
					// We only verify that they are created in the WantCreates.
					WithBrokerCellIngressFailed("EndpointsUnavailable", `Endpoints "test-brokercell-brokercell-ingress" is unavailable.`),
					WithBrokerCellFanoutUnknown("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-fanout" is unavailable.`),
					WithBrokerCellRetryUnknown("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-retry" is unavailable.`),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellSetDefaults,
				)},
			},
			WantEvents: []string{
				configmapCreatedEvent,
				ingressDeploymentCreatedEvent,
				ingressHPACreatedEvent,
				ingressServiceCreatedEvent,
				fanoutDeploymentCreatedEvent,
				fanoutHPACreatedEvent,
				retryDeploymentCreatedEvent,
				retryHPACreatedEvent,
				brokerCellReconciledEvent,
			},
		},
		{
			Name: "BrokerCell created, resources updated but some resource status not ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				NewBroker("broker", testNS, WithBrokerSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS),
				NewDeployment(brokerCellName+"-brokercell-ingress", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.IngressDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.IngressDeployment(t).ObjectMeta
					},
				),
				NewService(brokerCellName+"-brokercell-ingress", testNS,
					func(s *corev1.Service) {
						s.TypeMeta = testingdata.IngressService(t).TypeMeta
						s.ObjectMeta = testingdata.IngressService(t).ObjectMeta
					}),
				NewDeployment(brokerCellName+"-brokercell-fanout", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.FanoutDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.FanoutDeployment(t).ObjectMeta
					},
				),
				NewDeployment(brokerCellName+"-brokercell-retry", testNS,
					func(d *appsv1.Deployment) {
						d.TypeMeta = testingdata.RetryDeployment(t).TypeMeta
						d.ObjectMeta = testingdata.RetryDeployment(t).ObjectMeta
					},
				),
				emptyHPASpec(testingdata.IngressHPA(t)),
				emptyHPASpec(testingdata.FanoutHPA(t)),
				emptyHPASpec(testingdata.RetryHPA(t)),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{Object: testingdata.Config(t,
					NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
					NewBroker("broker", testNS, WithBrokerSetDefaults))},
				{Object: testingdata.IngressDeployment(t)},
				{Object: testingdata.IngressHPA(t)},
				{Object: testingdata.IngressService(t)},
				{Object: testingdata.FanoutDeployment(t)},
				{Object: testingdata.FanoutHPA(t)},
				{Object: testingdata.RetryDeployment(t)},
				{Object: testingdata.RetryHPA(t)},
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					// optimistically set everything to be ready, the following options will override individual conditions
					WithBrokerCellReady,
					// For newly created deployments and services, there statues are not ready because
					// we don't have a controller in the tests to mark their statues ready.
					// We only verify that they are created in the WantCreates.
					WithBrokerCellIngressFailed("EndpointsUnavailable", `Endpoints "test-brokercell-brokercell-ingress" is unavailable.`),
					WithBrokerCellFanoutUnknown("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-fanout" is unavailable.`),
					WithBrokerCellRetryUnknown("DeploymentUnavailable", `Deployment "test-brokercell-brokercell-retry" is unavailable.`),
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellSetDefaults,
				)},
			},
			WantEvents: []string{
				configmapUpdatedEvent,
				ingressDeploymentUpdatedEvent,
				ingressHPAUpdatedEvent,
				ingressServiceUpdatedEvent,
				fanoutDeploymentUpdatedEvent,
				fanoutHPAUpdatedEvent,
				retryDeploymentUpdatedEvent,
				retryHPAUpdatedEvent,
				brokerCellReconciledEvent,
			},
		},
		{
			Name: "BrokerCell created successfully but status update failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
				testingdata.RetryHPA(t),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "brokercells"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellReady,
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellSetDefaults,
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
				NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
				testingdata.EmptyConfig(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
				testingdata.RetryHPA(t),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellReady,
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellSetDefaults,
				)},
			},
			WantEvents: []string{
				brokerCellReconciledEvent,
			},
		},
		{
			Name: "googlecloud created BrokerCell shouldn't be gc'ed because there are brokers",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellAnnotations(creatorAnnotation),
					WithBrokerCellSetDefaults),
				testingdata.Config(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
					NewBroker("broker", testNS, WithBrokerSetDefaults)),
				NewBroker("broker", testNS, WithBrokerSetDefaults),
				NewEndpoints(brokerCellName+"-brokercell-ingress", testNS,
					WithEndpointsAddresses(corev1.EndpointAddress{IP: "127.0.0.1"})),
				testingdata.IngressDeploymentWithStatus(t),
				testingdata.IngressServiceWithStatus(t),
				testingdata.FanoutDeploymentWithStatus(t),
				testingdata.RetryDeploymentWithStatus(t),
				testingdata.IngressHPA(t),
				testingdata.FanoutHPA(t),
				testingdata.RetryHPA(t),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{Object: NewBrokerCell(brokerCellName, testNS,
					WithBrokerCellAnnotations(creatorAnnotation),
					WithBrokerCellReady,
					WithIngressTemplate("http://test-brokercell-brokercell-ingress.testnamespace.svc.cluster.local/{namespace}/{name}"),
					WithBrokerCellSetDefaults,
				)},
			},
			WantEvents: []string{
				brokerCellReconciledEvent,
			},
		},
		{
			Name: "googlecloud created BrokerCell should be gc'ed if there is no broker, but deletion fails",
			Key:  testKey,
			Objects: []runtime.Object{NewBrokerCell(brokerCellName, testNS,
				WithBrokerCellAnnotations(creatorAnnotation),
				WithBrokerCellSetDefaults,
				WithInitBrokerCellConditions,
			)},
			WithReactors: []clientgotesting.ReactionFunc{InduceFailure("delete", "brokercells")},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					Name: brokerCellName,
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Verb:      "delete",
						Resource:  intv1alpha1.SchemeGroupVersion.WithResource("brokercells"),
					},
				},
			},
			WantEvents: []string{brokerCellGCFailedEvent},
			WantErr:    true,
		},
		{
			Name: "googlecloud created BrokerCell is gc'ed successfully",
			Key:  testKey,
			Objects: []runtime.Object{NewBrokerCell(brokerCellName, testNS,
				WithBrokerCellAnnotations(creatorAnnotation),
				WithBrokerCellSetDefaults,
				WithInitBrokerCellConditions,
			)},
			WantDeletes: []clientgotesting.DeleteActionImpl{
				{
					Name: brokerCellName,
					ActionImpl: clientgotesting.ActionImpl{
						Namespace: testNS,
						Verb:      "delete",
						Resource:  intv1alpha1.SchemeGroupVersion.WithResource("brokercells"),
					},
				},
			},
			WantEvents: []string{brokerCellGCEvent},
		},
	}

	table.Test(t, MakeFactory(func(ctx context.Context, testingListers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		setReconcilerEnv()
		base := reconciler.NewBase(ctx, controllerAgentName, cmw)
		ls := listers{
			brokerLister:     testingListers.GetBrokerLister(),
			hpaLister:        testingListers.GetHPALister(),
			triggerLister:    testingListers.GetTriggerLister(),
			configMapLister:  testingListers.GetConfigMapLister(),
			serviceLister:    testingListers.GetK8sServiceLister(),
			endpointsLister:  testingListers.GetEndpointsLister(),
			deploymentLister: testingListers.GetDeploymentLister(),
			podLister:        testingListers.GetPodLister(),
		}

		r, err := NewReconciler(base, ls)
		if err != nil {
			t.Fatalf("Failed to created BrokerCell reconciler: %v", err)
		}
		return bcreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, testingListers.GetBrokerCellLister(), r.Recorder, r)
	}))
}

func emptyHPASpec(template *hpav2beta2.HorizontalPodAutoscaler) *hpav2beta2.HorizontalPodAutoscaler {
	template.Spec = hpav2beta2.HorizontalPodAutoscalerSpec{}
	return template
}

// The unit test to test when the brokerCell created successfully, the broker targets config should be updated with broker
// and trigger. Since the serialization order of the binary data of brokerTargets in the configMap is not guaranteed, we need
// to deserialization the binary data to a brokerTargets proto to compare, so it should be rewritten without using the tableTest Utility.
func TestBrokerTargetsReconcileConfig(t *testing.T) {
	setReconcilerEnv()
	bc := NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults)
	objects := []runtime.Object{
		bc,
		NewBroker("broker", testNS, WithBrokerSetDefaults),
		NewTrigger("trigger1", testNS, "broker", WithTriggerSetDefaults),
		NewTrigger("trigger2", testNS, "broker", WithTriggerSetDefaults),
	}
	ctx, _ := SetupFakeContext(t)
	cmw := configmap.NewStaticWatcher()
	ctx, client := fakekubeclient.With(ctx)
	base := reconciler.NewBase(ctx, controllerAgentName, cmw)
	testingListers := NewListers(objects)
	ls := listers{
		brokerLister:     testingListers.GetBrokerLister(),
		hpaLister:        testingListers.GetHPALister(),
		triggerLister:    testingListers.GetTriggerLister(),
		configMapLister:  testingListers.GetConfigMapLister(),
		serviceLister:    testingListers.GetK8sServiceLister(),
		endpointsLister:  testingListers.GetEndpointsLister(),
		deploymentLister: testingListers.GetDeploymentLister(),
		podLister:        testingListers.GetPodLister(),
	}
	r, err := NewReconciler(base, ls)
	if err != nil {
		t.Fatalf("Failed to create BrokerCell reconciler: %v", err)
	}
	// here we only want to test the functionality of the reconcileConfig that it should create a brokerTargets config successfully
	r.reconcileConfig(ctx, bc)
	wantMap := testingdata.Config(t, NewBrokerCell(brokerCellName, testNS, WithBrokerCellSetDefaults),
		NewBroker("broker", testNS, WithBrokerSetDefaults),
		NewTrigger("trigger1", testNS, "broker", WithTriggerSetDefaults),
		NewTrigger("trigger2", testNS, "broker", WithTriggerSetDefaults))
	gotMap, err := client.CoreV1().ConfigMaps(testNS).Get(context.Background(), resources.Name(bc.Name, targetsCMName), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap from client: %v", err)
	}
	// compare the ObjectMeta field
	if diff := cmp.Diff(wantMap.ObjectMeta, gotMap.ObjectMeta); diff != "" {
		t.Fatalf("Unexpected ObjectMeta in ConfigMap(-want, +got): %s", diff)
	}
	// deserialize the binary data to a broker targets config proto
	var wantBrokerTargets config.TargetsConfig
	var gotBrokerTargets config.TargetsConfig
	if err := proto.Unmarshal(wantMap.BinaryData[targetsCMKey], &wantBrokerTargets); err != nil {
		t.Fatalf("Failed to deserialize the binary data in ConfigMap: %v", err)
	}
	if err := proto.Unmarshal(gotMap.BinaryData[targetsCMKey], &gotBrokerTargets); err != nil {
		t.Fatalf("Failed to deserialize the binary data in ConfigMap: %v", err)
	}
	// compare the broker targets config
	if diff := cmp.Diff(wantBrokerTargets.String(), gotBrokerTargets.String()); diff != "" {
		t.Fatalf("Unexpected brokerTargets in ConfigMap(-want, +got): %s", diff)
	}
}
