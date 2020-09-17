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

package utils

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/ptr"
)

const (
	deploymentCreated = "DeploymentCreated"
	deploymentUpdated = "DeploymentUpdated"
)

type DeploymentReconciler struct {
	KubeClient kubernetes.Interface
	Lister     appsv1listers.DeploymentLister
	Recorder   record.EventRecorder
}

// ReconcileDeployment reconciles the K8s Deployment 'd'.
func (r *DeploymentReconciler) ReconcileDeployment(ctx context.Context, obj runtime.Object, d *appsv1.Deployment) (*appsv1.Deployment, error) {
	current, err := r.Lister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
		if apierrs.IsAlreadyExists(err) {
			return current, nil
		}
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, deploymentCreated, "Created deployment %s/%s", d.Namespace, d.Name)
		}
		return current, err
	}
	if err != nil {
		return nil, err
	}
	// Don't reconcile on replica difference.
	if current.Spec.Replicas != nil {
		d.Spec.Replicas = ptr.Int32(*current.Spec.Replicas)
	}
	if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		d, err := r.KubeClient.AppsV1().Deployments(desired.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, deploymentUpdated, "Updated deployment %s/%s", d.Namespace, d.Name)
		}
		return d, err
	}
	return current, err
}
