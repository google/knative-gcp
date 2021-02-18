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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// list.go contains functions to get a list of resources based on label selector and get information from a list of resources.
package utils

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

// GetPodList get a list of Pods in a certain namespace with certain label selector.
func GetPodList(ctx context.Context, ls labels.Selector, kubeClientSet kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	pl, err := kubeClientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ls.String(),
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
	})
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list pod", zap.Error(err))
		return nil, err
	}
	return pl, nil
}
