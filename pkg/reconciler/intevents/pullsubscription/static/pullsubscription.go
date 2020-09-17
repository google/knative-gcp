/*
Copyright 2019 Google LLC

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

package static

import (
	"context"

	"go.uber.org/zap"

	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	pullsubscriptionreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1/pullsubscription"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for PullSubscription resources.
type Reconciler struct {
	*psreconciler.Base
}

// Check that our Reconciler implements Interface.
var _ pullsubscriptionreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, ps *v1.PullSubscription) reconciler.Event {
	return r.Base.ReconcileKind(ctx, ps)
}

func (r *Reconciler) ReconcileDeployment(ctx context.Context, ra *appsv1.Deployment, src *v1.PullSubscription) error {
	existing, err := r.Base.GetOrCreateReceiveAdapter(ctx, ra, src)
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(ra.Spec, existing.Spec) {
		existing.Spec = ra.Spec
		existing, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			src.Status.MarkDeployedFailed("ReceiveAdapterUpdateFailed", "Error updating the Receive Adapter: %s", err.Error())
			logging.FromContext(ctx).Desugar().Error("Error updating Receive Adapter", zap.Error(err))
			return err
		}
	}

	src.Status.PropagateDeploymentAvailability(existing)
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, ps *v1.PullSubscription) reconciler.Event {
	return r.Base.FinalizeKind(ctx, ps)
}
