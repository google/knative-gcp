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

package resources

import (
	"fmt"

	"go.uber.org/multierr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// UpdateVolumeGenerationForDeployment updates the volume generation annotation
// for all pods in this deployment. This technique is used to trigger immediate
// configmap volume refresh without restarting the pod, see
// https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#mounted-configmaps-are-updated-automatically
// NOTE: update spec.template.metadata.annotations on the deployment resource
// causes pods to bre recreated.
func UpdateVolumeGenerationForDeployment(kubeClient kubernetes.Interface, dl appsv1listers.DeploymentLister, pl corev1listers.PodLister, ns, name string) error {
	d, err := dl.Deployments(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return fmt.Errorf("deployment %v/%v doesn't exist: %w", ns, name, err)
		}
		return fmt.Errorf("error getting deployment %v/%v: %w", ns, name, err)
	}

	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return fmt.Errorf("error converting deployment selector to label selector for %v/%v: %w", ns, name, err)
	}
	pods, err := pl.Pods(ns).List(selector)
	if err != nil {
		return fmt.Errorf("error listing pods for deployment %v/%v: %w", ns, name, err)
	}

	for _, pod := range pods {
		err = multierr.Append(err, UpdateVolumeGeneration(kubeClient, pod))
	}
	return err
}
