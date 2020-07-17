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
	"github.com/golang/protobuf/proto"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	pkgTest "knative.dev/pkg/test"
)

func PubSubTargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "pubsub_target", envVars)
}

func BuildTargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "build_target", envVars)
}

func StorageTargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "storage_target", envVars)
}

func AuditLogsTargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "auditlogs_target", envVars)
}

func SchedulerTargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "scheduler_target", envVars)
}

func TargetJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "target", envVars)
}

func SenderJob(name string, envVars []corev1.EnvVar) *batchv1.Job {
	return baseJob(name, "sender", envVars)
}

// baseJob will return a base Job that has imageName and envVars set for its PodTemplateSpec.
func baseJob(name, imageName string, envVars []corev1.EnvVar) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  proto.Int32(1),
			BackoffLimit: proto.Int32(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"e2etest": string(uuid.NewUUID()),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            name,
						Image:           pkgTest.ImagePath(imageName),
						ImagePullPolicy: corev1.PullAlways,
						Env:             envVars,
					}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
