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

package volume

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// volumeGenerationKey is the annotation key for broker data plane pods whose
// value is updated upon broker-targets configmap update. This is used to
// trigger immediate configmap volume refresh without restarting the pod.
const volumeGenerationKey = "volumeGeneration"

// UpdateVolumeGeneration updates the volume generation annotation
// for all pods matching the selector. This technique is used to trigger immediate
// configmap volume refresh without restarting the pod, see
// https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#mounted-configmaps-are-updated-automatically
// NOTE: update spec.template.metadata.annotations on the deployment resource
// causes pods to be recreated.
func UpdateVolumeGeneration(ctx context.Context, kubeClient kubernetes.Interface, pl corev1listers.PodLister, ns string, ls map[string]string) error {
	pods, err := pl.Pods(ns).List(labels.SelectorFromSet(ls))
	if err != nil {
		return fmt.Errorf("error listing pods in namespace %v with labels %v: %w", ns, ls, err)
	}

	for _, pod := range pods {
		err = multierr.Append(err, updateVolumeGeneration(ctx, kubeClient, pod))
	}
	return err
}

// updateVolumeGeneration updates the volume generation annotation on the pod to
// trigger volume mount refresh (for configmap).
func updateVolumeGeneration(ctx context.Context, kubeClientSet kubernetes.Interface, p *corev1.Pod) error {
	pod := p.DeepCopy()
	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}

	// TODO(https://github.com/google/knative-gcp/issues/913) Controller updates
	// generation on the configmap, and use configmap generation as annotation.
	gen, _ := strconv.Atoi(annotations[volumeGenerationKey])
	annotations[volumeGenerationKey] = strconv.Itoa(gen + 1)
	pod.SetAnnotations(annotations)

	if _, err := kubeClientSet.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating annotations for pod %v/%v: %w", pod.Namespace, pod.Name, err)
	}
	return nil
}
