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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapOption enables further configuration of a ConfigMap.
type ConfigMapOption func(*corev1.ConfigMap)

// NewConfigMap creates a ConfigMap with ConfigMapOptions.
func NewConfigMap(name, namespace string, cmo ...ConfigMapOption) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range cmo {
		opt(cm)
	}
	return cm
}

func WithConfigMapData(data map[string]string) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		cm.Data = data
	}
}

func WithConfigMapBinaryData(data map[string][]byte) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		cm.BinaryData = data
	}
}

func WithConfigMapDataEntry(key, value string) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[key] = value
	}
}

func WithConfigMapBinaryDataEntry(key string, value []byte) ConfigMapOption {
	return func(cm *corev1.ConfigMap) {
		if cm.BinaryData == nil {
			cm.BinaryData = make(map[string][]byte)
		}
		cm.BinaryData[key] = value
	}
}
