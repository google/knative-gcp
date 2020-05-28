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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/names"
	pkgreconciler "knative.dev/pkg/reconciler"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	brokerlisters "github.com/google/knative-gcp/pkg/client/listers/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"github.com/google/knative-gcp/pkg/reconciler/brokercell/resources"
)

type envConfig struct {
	IngressImage       string `envconfig:"INGRESS_IMAGE" required:"true"`
	FanoutImage        string `envconfig:"FANOUT_IMAGE" required:"true"`
	RetryImage         string `envconfig:"RETRY_IMAGE" required:"true"`
	ServiceAccountName string `envconfig:"SERVICE_ACCOUNT" default:"broker"`
	IngressPort        int    `envconfig:"INGRESS_PORT" default:"8080"`
	MetricsPort        int    `envconfig:"METRICS_PORT" default:"9090"`
}

// NewReconciler creates a new BrokerCell reconciler.
func NewReconciler(base *reconciler.Base, brokerLister brokerlisters.BrokerLister, serviceLister corev1listers.ServiceLister, endpointsLister corev1listers.EndpointsLister, deploymentLister appsv1listers.DeploymentLister) (*Reconciler, error) {
	var env envConfig
	if err := envconfig.Process("BROKER_CELL", &env); err != nil {
		return nil, err
	}
	svcRec := &reconciler.ServiceReconciler{
		KubeClient:      base.KubeClientSet,
		ServiceLister:   serviceLister,
		EndpointsLister: endpointsLister,
		Recorder:        base.Recorder,
	}
	deploymentRec := &reconciler.DeploymentReconciler{
		KubeClient: base.KubeClientSet,
		Lister:     deploymentLister,
		Recorder:   base.Recorder,
	}
	r := &Reconciler{
		Base:             base,
		brokerLister:     brokerLister,
		serviceLister:    serviceLister,
		endpointsLister:  endpointsLister,
		deploymentLister: deploymentLister,
		env:              env,
		svcRec:           svcRec,
		deploymentRec:    deploymentRec,
	}
	return r, nil
}

// Reconciler implements controller.Reconciler for BrokerCell resources.
type Reconciler struct {
	*reconciler.Base

	brokerLister     brokerlisters.BrokerLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
	deploymentLister appsv1listers.DeploymentLister

	svcRec        *reconciler.ServiceReconciler
	deploymentRec *reconciler.DeploymentReconciler

	env envConfig
}

// Check that our Reconciler implements Interface
var _ bcreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	if r.shouldGC(ctx, bc) {
		logging.FromContext(ctx).Info("Garbage collecting brokercell",  zap.String("brokercell", bc.Name), zap.String("Namespace", bc.Namespace))
		return r.delete(ctx, bc)
	}

	bc.Status.InitializeConditions()

	// Reconcile ingress deployment and service
	ingressArgs := r.makeIngressArgs(bc)
	if _, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeIngressDeployment(ingressArgs)); err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile ingress deployment for \"%s/%s\": %v", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkIngressFailed("IngressDeploymentFailed", "Failed to reconcile ingress deployment: %v", err)
		return err
	}
	endpoints, err := r.svcRec.ReconcileService(bc, resources.MakeIngressService(ingressArgs))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile ingress service for \"%s/%s\": %v", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkIngressFailed("IngressServiceFailed", "Failed to reconcile ingress service: %v", err)
		return err
	}
	bc.Status.PropagateIngressAvailability(endpoints)
	hostName := names.ServiceHostName(endpoints.GetName(), endpoints.GetNamespace())
	bc.Status.IngressTemplate = fmt.Sprintf("http://%s/{namespace}/{name}", hostName)
	// Reconcile fanout deployment
	fd, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeFanoutDeployment(r.makeFanoutArgs(bc)))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile fanout deployment for \"%s/%s\": %v", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkFanoutFailed("FanoutDeploymentFailed", "Failed to reconcile fanout deployment: %v", err)
		return err
	}
	bc.Status.PropagateFanoutAvailability(fd)
	// Reconcile retry deployment
	rd, err := r.deploymentRec.ReconcileDeployment(bc, resources.MakeRetryDeployment(r.makeRetryArgs(bc)))
	if err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile retry deployment for \"%s/%s\": %v", zap.Any("namespace", bc.Namespace), zap.Any("name", bc.Name), zap.Error(err))
		bc.Status.MarkRetryFailed("RetryDeploymentFailed", "Failed to reconcile retry deployment: %v", err)
		return err
	}
	bc.Status.PropagateRetryAvailability(rd)

	// TODO Reconcile:
	// - Configmap
	bc.Status.MarkTargetsConfigReady()

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
		},
		Port: r.env.IngressPort,
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
		},
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
		},
	}
}
