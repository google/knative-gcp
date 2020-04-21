/*
Copyright 2020 Google LLC.

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

type ConfigMapOption func(*corev1.ConfigMap)

func NewConfigMap(name, namespace string, opts ...ConfigMapOption) *corev1.ConfigMap {
	c := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithData(data map[string]string) ConfigMapOption {
	return func(c *corev1.ConfigMap) {
		c.Data = data
	}
}

func WithBinaryData(data map[string][]uint8) ConfigMapOption {
	return func(c *corev1.ConfigMap) {
		c.BinaryData = data
	}
}
