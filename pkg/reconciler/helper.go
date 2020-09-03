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

package reconciler

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	deploymentCreated = "DeploymentCreated"
	deploymentUpdated = "DeploymentUpdated"
	serviceCreated    = "ServiceCreated"
	serviceUpdated    = "ServiceUpdated"
	configMapCreated  = "ConfigMapCreated"
	configMapUpdated  = "ConfigMapUpdated"
)

type ServiceReconciler struct {
	KubeClient      kubernetes.Interface
	ServiceLister   corev1listers.ServiceLister
	EndpointsLister corev1listers.EndpointsLister
	Recorder        record.EventRecorder
}

type DeploymentReconciler struct {
	KubeClient kubernetes.Interface
	Lister     appsv1listers.DeploymentLister
	Recorder   record.EventRecorder
}

type ConfigMapReconciler struct {
	KubeClient kubernetes.Interface
	Lister     corev1listers.ConfigMapLister
	Recorder   record.EventRecorder
}

// ReconcileDeployment reconciles the K8s Deployment 'd'.
func (r *DeploymentReconciler) ReconcileDeployment(obj runtime.Object, d *appsv1.Deployment) (*appsv1.Deployment, error) {
	current, err := r.Lister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.AppsV1().Deployments(d.Namespace).Create(d)
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
	if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		d, err := r.KubeClient.AppsV1().Deployments(desired.Namespace).Update(desired)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, deploymentUpdated, "Updated deployment %s/%s", d.Namespace, d.Name)
		}
		return d, err
	}
	return current, err
}

// ReconcileService reconciles the K8s Service 'svc'.
func (r *ServiceReconciler) ReconcileService(obj runtime.Object, svc *corev1.Service) (*corev1.Endpoints, error) {
	current, err := r.ServiceLister.Services(svc.Namespace).Get(svc.Name)

	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.CoreV1().Services(svc.Namespace).Create(svc)
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
		current, err = r.KubeClient.CoreV1().Services(current.Namespace).Update(desired)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, serviceUpdated, "Updated service %s/%s", svc.Namespace, svc.Name)
		}
		if err != nil {
			return nil, err
		}
	}

	return r.EndpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}

// DefaultConfigMapEqual is a basic equality testing function for K8s ConfigMaps
// based on string Data.
func DefaultConfigMapEqual(cm1, cm2 *corev1.ConfigMap) bool {
	return equality.Semantic.DeepEqual(cm1.Data, cm2.Data)
}

// ReconcileConfigMap reconciles the K8s ConfigMap 'cm'.
func (r *ConfigMapReconciler) ReconcileConfigMap(obj runtime.Object, cm *corev1.ConfigMap, eqFunc func(*corev1.ConfigMap, *corev1.ConfigMap) bool, handlers ...cache.ResourceEventHandlerFuncs) (*corev1.ConfigMap, error) {
	current, err := r.Lister.ConfigMaps(cm.Namespace).Get(cm.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(cm)
		if apierrs.IsAlreadyExists(err) {
			return current, nil
		}
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, configMapCreated, "Created configmap %s/%s", cm.Namespace, cm.Name)
			for _, h := range handlers {
				h.OnAdd(current)
			}
		}
		return current, err
	}
	if err != nil {
		return nil, err
	}
	if eqFunc == nil {
		return nil, fmt.Errorf("unspecified ConfigMap equality testing function")
	}
	if !eqFunc(cm, current) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Data = cm.Data
		desired.BinaryData = cm.BinaryData
		res, err := r.KubeClient.CoreV1().ConfigMaps(desired.Namespace).Update(desired)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, configMapUpdated, "Updated configmap %s/%s", res.Namespace, res.Name)
			for _, h := range handlers {
				h.OnUpdate(current, desired)
			}
		}
		return res, err
	}
	return current, err
}
