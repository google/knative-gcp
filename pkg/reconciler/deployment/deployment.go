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
	"github.com/google/knative-gcp/pkg/reconciler"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"strconv"

	v1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
)

const (
	updatedSecretRevisionAnno = "api.v1.namespaces.cloud-run-events.secrets.google-cloud-key/updated-revision"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	deploymentLister appsv1listers.DeploymentLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconciler implements controller.Reconciler
// Reconciler get the deployment and then update the deployment's annotation.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}
	// Get the deployment resource with this namespace/name
	original, err := r.deploymentLister.Deployments(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Errorf("deployment key in work queue no longer exists %q %q", namespace, name)
		return nil
	} else if err != nil {
		logging.FromContext(ctx).Errorf("Error fetching deployment with namespace %q name %q: error", namespace, name, err)
		return err
	}

	d := original.DeepCopy()

	// Reconcile this copy of the Deployment.
	return r.reconcile(ctx, d)
}

func (r *Reconciler) reconcile(ctx context.Context, d *v1.Deployment) error {
	if d.DeletionTimestamp != nil {
		return nil
	}

	anno := d.Spec.Template.GetObjectMeta().GetAnnotations()
	revision, ok := anno[updatedSecretRevisionAnno]
	if !ok {
		anno = make(map[string]string)
		anno[updatedSecretRevisionAnno] = "1"
	} else {
		i, err := strconv.Atoi(revision)
		if err != nil {
			return fmt.Errorf("error converting the %q of annotation %q of deployment of namespace %q, name %q from string to int", revision, updatedSecretRevisionAnno, d.Namespace, d.Name)
		}
		anno[updatedSecretRevisionAnno] = strconv.Itoa(i + 1)
	}
	d.Spec.Template.SetAnnotations(anno)
	_, err := r.KubeClientSet.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}
	return nil
}
