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
	context "context"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for BrokerCell resources.
type Reconciler struct {
	*reconciler.Base
	// TODO: add additional requirements here.
}

// Check that our Reconciler implements Interface
var _ bcreconciler.Interface = (*Reconciler)(nil)

// Optionally check that our Reconciler implements Finalizer
var _ bcreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	bc.Status.InitializeConditions()

	// TODO Reconcile:
	// - Ingress service
	// - Ingress deployment
	// - Fanout deployment
	// - Retry deployment
	// - Configmap

	bc.Status.ObservedGeneration = bc.Generation
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellReconciled", "BrokerCell reconciled: \"%s/%s\"", bc.Namespace, bc.Name)
}

// FinalizeKind will be called when the resource is deleted.
func (r *Reconciler) FinalizeKind(ctx context.Context, bc *intv1alpha1.BrokerCell) pkgreconciler.Event {
	// TODO Finalize by:
	// - Deleting the deployments. Wait until they're gone
	// - Delete the configmap
	// - When to delete the svc? Look at revision deletion to see if it specifies a deletion order
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "BrokerCellFinalized", "BrokerCell finalized: \"%s/%s\"", bc.Namespace, bc.Name)
}
