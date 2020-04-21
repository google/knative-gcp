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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// volumeGenerationKey is the annotation key for broker data plane pods whose
// value is updated upon broker-targets configmap update. This is used to
// trigger immediate configmap volume refresh without restarting the pod.
const volumeGenerationKey = "volumeGeneration"

// UpdateVolumeGeneration updates the volume generation annotation on the pod to
// trigger volume mount refresh (for configmap).
func UpdateVolumeGeneration(kubeClientSet kubernetes.Interface, p *corev1.Pod) error {
	pod := p.DeepCopy()
	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	gen, _ := strconv.Atoi(annotations[volumeGenerationKey])
	annotations[volumeGenerationKey] = strconv.Itoa(gen + 1)
	pod.SetAnnotations(annotations)

	if _, err := kubeClientSet.CoreV1().Pods(pod.Namespace).Update(pod); err != nil {
		return fmt.Errorf("error updating annotations for pod %v/%v: %w", pod.Namespace, pod.Name, err)
	}
	return nil
}
