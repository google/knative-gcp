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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	serviceCreated = "ServiceCreated"
	serviceUpdated = "ServiceUpdated"
)

type ServiceReconciler struct {
	KubeClient      kubernetes.Interface
	ServiceLister   corev1listers.ServiceLister
	EndpointsLister corev1listers.EndpointsLister
	Recorder        record.EventRecorder
}

// ReconcileService reconciles the K8s Service 'svc'.
func (r *ServiceReconciler) ReconcileService(ctx context.Context, obj runtime.Object, svc *corev1.Service) (*corev1.Endpoints, error) {
	current, err := r.ServiceLister.Services(svc.Namespace).Get(svc.Name)

	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, serviceCreated, "Created service %s/%s", svc.Namespace, svc.Name)
		}
		if err == nil || apierrs.IsAlreadyExists(err) {
			return r.EndpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = svc.Spec
		current, err = r.KubeClient.CoreV1().Services(current.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, serviceUpdated, "Updated service %s/%s", svc.Namespace, svc.Name)
		}
		if err != nil {
			return nil, err
		}
	}

	return r.EndpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}
