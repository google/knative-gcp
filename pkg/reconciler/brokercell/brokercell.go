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

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	hpav2beta2listers "k8s.io/client-go/listers/autoscaling/v2beta2"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/names"
	pkgreconciler "knative.dev/pkg/reconciler"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	brokerlisters "github.com/google/knative-gcp/pkg/client/listers/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
	reconcilerutils "github.com/google/knative-gcp/pkg/reconciler/utils"
)

type envConfig struct {
	IngressImage           string `envconfig:"INGRESS_IMAGE" required:"true"`
	FanoutImage            string `envconfig:"FANOUT_IMAGE" required:"true"`
	RetryImage             string `envconfig:"RETRY_IMAGE" required:"true"`
	ServiceAccountName     string `envconfig:"SERVICE_ACCOUNT" default:"broker"`
	IngressPort            int    `envconfig:"INGRESS_PORT" default:"8080"`
	MetricsPort            int    `envconfig:"METRICS_PORT" default:"9090"`
	InternalMetricsEnabled bool   `envconfig:"INTERNAL_METRICS_ENABLED" default:"false"`
}

type listers struct {
	brokerLister     brokerlisters.BrokerLister
	hpaLister        hpav2beta2listers.HorizontalPodAutoscalerLister
	triggerLister    brokerlisters.TriggerLister
	configMapLister  corev1listers.ConfigMapLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
	deploymentLister appsv1listers.DeploymentLister
	podLister        corev1listers.PodLister
}

// NewReconciler creates a new BrokerCell reconciler.
func NewReconciler(base *reconciler.Base, ls listers) (*Reconciler, error) {
	var env envConfig
	if err := envconfig.Process("BROKER_CELL", &env); err != nil {
		return nil, err
	}
	svcRec := &reconcilerutils.ServiceReconciler{
		KubeClient:      base.KubeClientSet,
		ServiceLister:   ls.serviceLister,
		EndpointsLister: ls.endpointsLister,
		Recorder:        base.Recorder,
	}
	deploymentRec := &reconcilerutils.DeploymentReconciler{
		KubeClient: base.KubeClientSet,
		Lister:     ls.deploymentLister,
		Recorder:   base.Recorder,
	}
	cmRec := &reconcilerutils.ConfigMapReconciler{
		KubeClient: base.KubeClientSet,
		Lister:     ls.configMapLister,
		Recorder:   base.Recorder,
	}
	r := &Reconciler{
		Base:          base,
		env:           env,
		listers:       ls,
		svcRec:        svcRec,
		deploymentRec: deploymentRec,
		cmRec:         cmRec,
	}
	return r, nil
}

// Reconciler implements controller.Reconciler for BrokerCell resources.
type Reconciler struct {
	*reconciler.Base

	listers

	svcRec        *reconcilerutils.ServiceReconciler
	deploymentRec *reconcilerutils.DeploymentReconciler
	cmRec         *reconcilerutils.ConfigMapReconciler

	env envConfig
}

// Check that our Reconciler implements Interface
var _ bcreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	// Why are we doing GC here instead of in the broker controller?
	// 1. It's tricky to handle concurrency in broker controller. Suppose you are deleting all
	// brokers at the same time, hard to tell if the brokercell should be gc'ed.
	// 2. It's also more reliable. If for some reason we didn't delete the brokercell in the broker
	// controller when we should (due to race, missing event, bug, etc), and if all the brokers are
	// deleted, then we don't have a chance to retry.
	// TODO(https://github.com/google/knative-gcp/issues/1196) It's cleaner to make this a separate controller.
	if r.shouldGC(ctx, bc) {
		logging.FromContext(ctx).Info("Garbage collecting brokercell", zap.String("brokercell", bc.Name), zap.String("Namespace", bc.Namespace))
		return r.delete(ctx, bc)
	}

	bc.Status.InitializeConditions()

	// Reconcile broker targets configmap first so that data plane pods are guaranteed to have the configmap volume
	// mount available.
	if err := r.reconcileConfig(ctx, bc); err != nil {
		return err
	}

	// Reconcile ingress deployment, HPA and service.
	ingressArgs := r.makeIngressArgs(bc)
	ind, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeIngressDeployment(ingressArgs))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile ingress deployment", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkIngressFailed("IngressDeploymentFailed", "Failed to reconcile ingress deployment: %v", err)
		return err
	}

	ingressHPA := resources.MakeHorizontalPodAutoscaler(ind, r.makeIngressHPAArgs(bc))
	if err := r.reconcileAutoscaling(ctx, bc, ingressHPA); err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile ingress HPA", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkIngressFailed("HorizontalPodAutoscalerFailed", "Failed to reconcile ingress HorizontalPodAutoscaler: %v", err)
		return err
	}

	endpoints, err := r.svcRec.ReconcileService(bc, resources.MakeIngressService(ingressArgs))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile ingress service", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkIngressFailed("IngressServiceFailed", "Failed to reconcile ingress service: %v", err)
		return err
	}
	bc.Status.PropagateIngressAvailability(endpoints)
	hostName := names.ServiceHostName(endpoints.GetName(), endpoints.GetNamespace())
	bc.Status.IngressTemplate = fmt.Sprintf("http://%s/{namespace}/{name}", hostName)

	// Reconcile fanout deployment and HPA.
	fd, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeFanoutDeployment(r.makeFanoutArgs(bc)))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile fanout deployment", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkFanoutFailed("FanoutDeploymentFailed", "Failed to reconcile fanout deployment: %v", err)
		return err
	}

	fanoutHPA := resources.MakeHorizontalPodAutoscaler(fd, r.makeFanoutHPAArgs(bc))
	if err := r.reconcileAutoscaling(ctx, bc, fanoutHPA); err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile fanout HPA", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkFanoutFailed("HorizontalPodAutoscalerFailed", "Failed to reconcile fanout HorizontalPodAutoscaler: %v", err)
		return err
	}
	bc.Status.PropagateFanoutAvailability(fd)

	// Reconcile retry deployment and HPA.
	rd, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeRetryDeployment(r.makeRetryArgs(bc)))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile retry deployment", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkRetryFailed("RetryDeploymentFailed", "Failed to reconcile retry deployment: %v", err)
		return err
	}

	retryHPA := resources.MakeHorizontalPodAutoscaler(rd, r.makeRetryHPAArgs(bc))
	if err := r.reconcileAutoscaling(ctx, bc, retryHPA); err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile retry HPA", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkRetryFailed("HorizontalPodAutoscalerFailed", "Failed to reconcile retry HorizontalPodAutoscaler: %v", err)
		return err
	}
	bc.Status.PropagateRetryAvailability(rd)

	bc.Status.ObservedGeneration = bc.Generation
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellReconciled", "BrokerCell reconciled: \"%s/%s\"", bc.Namespace, bc.Name)
}

// shouldGC returns true if
// 1. the brokercell was automatically created by GCP broker controller (with annotation
// internal.events.cloud.google.com/creator: googlecloud), and
// 2. there is no brokers pointing to it
func (r *Reconciler) shouldGC(ctx context.Context, bc *intv1alpha1.BrokerCell) bool {
	// TODO use the constants in #1132 once it's merged
	// We only garbage collect brokercells that were automatically created by the GCP broker controller.
	if bc.GetAnnotations()["internal.events.cloud.google.com/creator"] != "googlecloud" {
		return false
	}

	// TODO(#866) Only select brokers that point to this brokercell by label selector once the
	// webhook assigns the brokercell label, i.e.,
	// r.brokerLister.List(labels.SelectorFromSet(map[string]string{"brokercell":bc.Name, "brokercellns":bc.Namespace}))
	brokers, err := r.brokerLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Failed to list brokers, skipping garbage collection logic", zap.String("brokercell", bc.Name), zap.String("Namespace", bc.Namespace))
		return false
	}

	return len(brokers) == 0
}

func (r *Reconciler) delete(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	if err := r.RunClientSet.InternalV1alpha1().BrokerCells(bc.Namespace).Delete(bc.Name, nil); err != nil {
		return fmt.Errorf("failed to garbage collect brokercell: %w", err)
	}
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellGarbageCollected", "BrokerCell garbage collected: \"%s/%s\"", bc.Namespace, bc.Name)
}

func (r *Reconciler) makeIngressArgs(bc *intv1alpha1.BrokerCell) resources.IngressArgs {
	return resources.IngressArgs{
		Args: resources.Args{
			ComponentName:      resources.IngressName,
			BrokerCell:         bc,
			Image:              r.env.IngressImage,
			ServiceAccountName: r.env.ServiceAccountName,
			MetricsPort:        r.env.MetricsPort,
			AllowIstioSidecar:  true,
			CPURequest:         bc.Spec.Components.Ingress.CPURequest,
			CPULimit:           bc.Spec.Components.Ingress.CPULimit,
			MemoryRequest:      bc.Spec.Components.Ingress.MemoryRequest,
			MemoryLimit:        bc.Spec.Components.Ingress.MemoryLimit,
		},
		Port: r.env.IngressPort,
	}
}

func (r *Reconciler) makeIngressHPAArgs(bc *intv1alpha1.BrokerCell) resources.AutoscalingArgs {
	return resources.AutoscalingArgs{
		ComponentName:     resources.IngressName,
		BrokerCell:        bc,
		AvgCPUUtilization: bc.Spec.Components.Ingress.AvgCPUUtilization,
		AvgMemoryUsage:    bc.Spec.Components.Ingress.AvgMemoryUsage,
		MaxReplicas:       *bc.Spec.Components.Ingress.MaxReplicas,
		MinReplicas:       *bc.Spec.Components.Ingress.MinReplicas,
	}
}

func (r *Reconciler) makeFanoutArgs(bc *intv1alpha1.BrokerCell) resources.FanoutArgs {
	return resources.FanoutArgs{
		Args: resources.Args{
			ComponentName:      resources.FanoutName,
			BrokerCell:         bc,
			Image:              r.env.FanoutImage,
			ServiceAccountName: r.env.ServiceAccountName,
			MetricsPort:        r.env.MetricsPort,
			CPURequest:         bc.Spec.Components.Fanout.CPURequest,
			CPULimit:           bc.Spec.Components.Fanout.CPULimit,
			MemoryRequest:      bc.Spec.Components.Fanout.MemoryRequest,
			MemoryLimit:        bc.Spec.Components.Fanout.MemoryLimit,
		},
	}
}

func (r *Reconciler) makeFanoutHPAArgs(bc *intv1alpha1.BrokerCell) resources.AutoscalingArgs {
	return resources.AutoscalingArgs{
		ComponentName:     resources.FanoutName,
		BrokerCell:        bc,
		AvgCPUUtilization: bc.Spec.Components.Fanout.AvgCPUUtilization,
		AvgMemoryUsage:    bc.Spec.Components.Fanout.AvgMemoryUsage,
		MaxReplicas:       *bc.Spec.Components.Fanout.MaxReplicas,
		MinReplicas:       *bc.Spec.Components.Fanout.MinReplicas,
	}
}

func (r *Reconciler) makeRetryArgs(bc *intv1alpha1.BrokerCell) resources.RetryArgs {
	return resources.RetryArgs{
		Args: resources.Args{
			ComponentName:      resources.RetryName,
			BrokerCell:         bc,
			Image:              r.env.RetryImage,
			ServiceAccountName: r.env.ServiceAccountName,
			MetricsPort:        r.env.MetricsPort,
			CPURequest:         bc.Spec.Components.Retry.CPURequest,
			CPULimit:           bc.Spec.Components.Retry.CPULimit,
			MemoryRequest:      bc.Spec.Components.Retry.MemoryRequest,
			MemoryLimit:        bc.Spec.Components.Retry.MemoryLimit,
		},
	}
}

func (r *Reconciler) makeRetryHPAArgs(bc *intv1alpha1.BrokerCell) resources.AutoscalingArgs {
	return resources.AutoscalingArgs{
		ComponentName:     resources.RetryName,
		BrokerCell:        bc,
		AvgCPUUtilization: bc.Spec.Components.Retry.AvgCPUUtilization,
		AvgMemoryUsage:    bc.Spec.Components.Retry.AvgMemoryUsage,
		MaxReplicas:       *bc.Spec.Components.Retry.MaxReplicas,
		MinReplicas:       *bc.Spec.Components.Retry.MinReplicas,
	}
}

func (r *Reconciler) reconcileAutoscaling(ctx context.Context, bc *intv1alpha1.BrokerCell, desired *hpav2beta2.HorizontalPodAutoscaler) error {
	existing, err := r.hpaLister.HorizontalPodAutoscalers(desired.Namespace).Get(desired.Name)
	if apierrs.IsNotFound(err) {
		existing, err = r.KubeClientSet.AutoscalingV2beta2().HorizontalPodAutoscalers(desired.Namespace).Create(desired)
		if apierrs.IsAlreadyExists(err) {
			return nil
		}
		if err == nil {
			r.Recorder.Eventf(bc, corev1.EventTypeNormal, "HorizontalPodAutoscalerCreated", "Created HPA %s/%s", desired.Namespace, desired.Name)
		}
		return err
	}
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepDerivative(desired.Spec, existing.Spec) {
		// Don't modify the informers copy.
		copy := existing.DeepCopy()
		copy.Spec = desired.Spec
		_, err := r.KubeClientSet.AutoscalingV2beta2().HorizontalPodAutoscalers(copy.Namespace).Update(copy)
		if err == nil {
			r.Recorder.Eventf(bc, corev1.EventTypeNormal, "HorizontalPodAutoscalerUpdated", "Updated HPA %s/%s", desired.Namespace, desired.Name)
		}
		return err
	}
	return nil
}
