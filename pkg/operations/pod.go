/*
Copyright 2019 Google LLC

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

package operations

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	credsVolume    = "google-cloud-key"
	credsMountPath = "/var/secrets/google"
)

// MakePodTemplate creates a pod template for a Job.
func MakePodTemplate(image string, secret corev1.SecretKeySelector, extEnv ...corev1.EnvVar) *corev1.PodTemplateSpec {
	credsFile := fmt.Sprintf("%s/%s", credsMountPath, secret.Key)
	env := []corev1.EnvVar{{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: credsFile,
	}}
	if len(extEnv) > 0 {
		env = append(env, extEnv...)
	}

	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "job",
				Image:           image,
				ImagePullPolicy: "Always",
				Env:             env,
				VolumeMounts: []corev1.VolumeMount{{
					Name:      credsVolume,
					MountPath: credsMountPath,
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: credsVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			}},
		},
	}
}

func GetFirstTerminationMessage(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
			return cs.State.Terminated.Message
		}
	}
	return ""
}
