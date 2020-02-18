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

package k8s

import (
	"context"
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	psreconciler "github.com/google/knative-gcp/pkg/reconciler/pubsub/pullsubscription"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// Reconciler implements controller.Reconciler for PullSubscription resources.
type Reconciler struct {
	*psreconciler.Base
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	return r.Base.Reconcile(ctx, key)
}

func (r *Reconciler) ReconcileDeployment(ctx context.Context, ra *appsv1.Deployment, src *v1alpha1.PullSubscription) error {
	existing, err := r.Base.GetOrCreateReceiveAdapter(ctx, ra, src)
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepDerivative(ra.Spec, existing.Spec) {
		existing.Spec = ra.Spec
		_, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(existing)
		if err != nil {
			logging.FromContext(ctx).Desugar().Error("Error updating Receive Adapter", zap.Error(err))
			return err
		}
	}
	return nil
}
