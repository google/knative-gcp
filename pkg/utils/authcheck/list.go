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
package authcheck

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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

// GetEventList get a list of k8s event in a certain namespace with certain field selector related to Pod.
func GetEventList(ctx context.Context, kubeClientSet kubernetes.Interface, pod, namespace string) (*corev1.EventList, error) {
	pl, err := kubeClientSet.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Event",
		},
		FieldSelector: podWarningFieldSelector(pod).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list kubernetes event", zap.Error(err))
		return nil, err
	}
	return pl, nil
}

// GetMountFailureMessageFromEventList gets the k8s events message that related to secret errors.
// It returns the first relevant k8s event message from any Event in the list.
func GetMountFailureMessageFromEventList(el *corev1.EventList, secret *corev1.SecretKeySelector) string {
	for _, event := range el.Items {
		if isSecretFailureMessage(event.Message, event.Namespace, secret) {
			return event.Message
		}
	}
	return ""
}

// GetTerminationLogFromPodList gets the termination log from Pods that failed due to authentication check errors.
// It returns the first authentication termination log from any Pods in the list.
func GetTerminationLogFromPodList(pl *corev1.PodList) string {
	for _, pod := range pl.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil && isAuthMessage(cs.State.Terminated.Message) {
				return cs.State.Terminated.Message
			} else if cs.LastTerminationState.Terminated != nil && isAuthMessage(cs.LastTerminationState.Terminated.Message) {
				return cs.LastTerminationState.Terminated.Message
			}
		}
	}
	return ""
}

func isAuthMessage(message string) bool {
	return strings.Contains(message, authMessage)
}

// isSecretFailureMessage checks if the message is for a specific secret's failure.
func isSecretFailureMessage(message, namespace string, secret *corev1.SecretKeySelector) bool {
	return strings.Contains(message, fmt.Sprintf(`secret "%s" not found`, secret.Name)) ||
		strings.Contains(message, fmt.Sprintf(`couldn't find key %s in Secret %s/%s`, secret.Key, namespace, secret.Name))
}

func podWarningFieldSelector(pod string) fields.Selector {
	return fields.SelectorFromSet(map[string]string{
		"involvedObject.kind": "Pod",
		"involvedObject.name": pod,
		"type":                "Warning",
	})
}
