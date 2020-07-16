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
package deployment

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"knative.dev/pkg/client/injection/kube/reconciler/apps/v1/deployment"
	"knative.dev/pkg/reconciler"
)

const (
	SecretUpdateAnnotation = "events.cloud.google.com/secretLastObservedUpdateTime"
)

// Reconciler implements controller.Reconciler for Deployment resources.
type Reconciler struct {
	clock clock.Clock

	// KubeClientSet allows us to talk to the k8s for core APIs.
	kubeClientSet kubernetes.Interface
}

// Check that our Reconciler implements Interface
var _ deployment.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
// It gets the deployment and then update its annotation.
// With this, the deployment will recreate the pods and they will pick up the latest secret image immediately.
// Otherwise we would need to wait for 1 min for the deployment pods to pick up the updated secret.
func (r *Reconciler) ReconcileKind(ctx context.Context, d *appsv1.Deployment) reconciler.Event {
	annotations := d.Spec.Template.GetObjectMeta().GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[SecretUpdateAnnotation] = r.clock.Now().String()
	d.Spec.Template.SetAnnotations(annotations)
	_, err := r.kubeClientSet.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}
	return nil
}
