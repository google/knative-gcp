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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/names"
	pkgreconciler "knative.dev/pkg/reconciler"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
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

// Reconciler implements controller.Reconciler for BrokerCell resources.
type Reconciler struct {
	*reconciler.Base

	kubeClientSet    kubernetes.Interface
	brokerLister     eventinglisters.BrokerLister
	serviceLister    corev1listers.ServiceLister
	endpointsLister  corev1listers.EndpointsLister
	deploymentLister appsv1listers.DeploymentLister

	env envConfig
}

// Check that our Reconciler implements Interface
var _ bcreconciler.Interface = (*Reconciler)(nil)

// Optionally check that our Reconciler implements Finalizer
var _ bcreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	bc.Status.InitializeConditions()

	// Reconcile ingress deployment and service
	ingressArgs := r.makeIngressArgs(bc)
	if _, err := reconciler.ReconcileDeployment(r.kubeClientSet, r.deploymentLister, resources.MakeIngressDeployment(ingressArgs)); err != nil {
		r.Logger.Errorf("Failed to reconcile ingress deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		bc.Status.MarkIngressFailed("IngressDeploymentFailed", "Failed to reconcile ingress deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		return err
	}
	endpoints, err := reconciler.ReconcileService(r.kubeClientSet, r.serviceLister, r.endpointsLister, resources.MakeIngressService(ingressArgs))
	if err != nil {
		r.Logger.Errorf("Failed to reconcile ingress service for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		bc.Status.MarkIngressFailed("IngressServiceFailed", "Failed to reconcile ingress service for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		return err
	}
	bc.Status.PropagateIngressAvailability(endpoints)
	bc.Status.IngressTemplate = "http://" + names.ServiceHostName(endpoints.GetName(), endpoints.GetNamespace())
	// Reconcile fanout deployment
	fd, err := reconciler.ReconcileDeployment(r.kubeClientSet, r.deploymentLister, resources.MakeFanoutDeployment(r.makeFanoutArgs(bc)))
	if err != nil {
		r.Logger.Errorf("Failed to reconcile fanout deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		bc.Status.MarkFanoutFailed("FanoutDeploymentFailed", "Failed to reconcile fanout deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		return err
	}
	bc.Status.PropagateFanoutAvailability(fd)
	// Reconcile retry deployment
	rd, err := reconciler.ReconcileDeployment(r.kubeClientSet, r.deploymentLister, resources.MakeRetryDeployment(r.makeRetryArgs(bc)))
	if err != nil {
		r.Logger.Errorf("Failed to reconcile retry deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		bc.Status.MarkRetryFailed("RetryDeploymentFailed", "Failed to reconcile retry deployment for \"%s/%s\": %v", bc.Namespace, bc.Name, err)
		return err
	}
	bc.Status.PropagateRetryAvailability(rd)

	// TODO Reconcile:
	// - Configmap
	bc.Status.MarkTargetsConfigReady()

	bc.Status.ObservedGeneration = bc.Generation
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellReconciled", "BrokerCell reconciled: \"%s/%s\"", bc.Namespace, bc.Name)
}

// FinalizeKind will be called when the resource is deleted.
func (r *Reconciler) FinalizeKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	// Resources owned by this brokercell should automatically be garbage collected.
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellFinalized", "BrokerCell finalized: \"%s/%s\"", bc.Namespace, bc.Name)
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
