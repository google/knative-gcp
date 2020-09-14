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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	configMapCreated = "ConfigMapCreated"
	configMapUpdated = "ConfigMapUpdated"
)

type ConfigMapReconciler struct {
	KubeClient kubernetes.Interface
	Lister     corev1listers.ConfigMapLister
	Recorder   record.EventRecorder
}

// DefaultConfigMapEqual is a basic equality testing function for K8s ConfigMaps
// based on string Data.
func DefaultConfigMapEqual(cm1, cm2 *corev1.ConfigMap) bool {
	return equality.Semantic.DeepEqual(cm1.Data, cm2.Data)
}

// ReconcileConfigMap reconciles the K8s ConfigMap 'cm'.
func (r *ConfigMapReconciler) ReconcileConfigMap(ctx context.Context, obj runtime.Object, cm *corev1.ConfigMap, eqFunc func(*corev1.ConfigMap, *corev1.ConfigMap) bool, handlers ...cache.ResourceEventHandlerFuncs) (*corev1.ConfigMap, error) {
	current, err := r.Lister.ConfigMaps(cm.Namespace).Get(cm.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
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
		res, err := r.KubeClient.CoreV1().ConfigMaps(desired.Namespace).Update(ctx, desired, metav1.UpdateOptions{})
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
