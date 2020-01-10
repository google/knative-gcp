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
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/uuid"
    pkgTest "knative.dev/pkg/test"
)

func TargetJob(name string) *batchv1.Job {
    const imageName = "target"
    return &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:   name,
        },
        Spec: batchv1.JobSpec{
            Parallelism:             proto.Int32(1),
            BackoffLimit:            proto.Int32(0),
            Template:                v1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "e2etest": string(uuid.NewUUID()),
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []v1.Container{{
                        Name:            imageName,
                        Image:           pkgTest.ImagePath(imageName),
                        ImagePullPolicy: corev1.PullAlways,
                        Env: []v1.EnvVar{{
                            Name:  "TARGET",
                            Value: "falldown",
                        }},
                    }},
                    RestartPolicy: corev1.RestartPolicyNever,
                },
            },
        },
    }
}

func SenderJob(name, url string) *batchv1.Job {
    const imageName = "sender"
    return &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:   name,
        },
        Spec: batchv1.JobSpec{
            Parallelism:             proto.Int32(1),
            BackoffLimit:            proto.Int32(0),
            Template:                v1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "e2etest": string(uuid.NewUUID()),
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []v1.Container{{
                        Name:            imageName,
                        Image:           pkgTest.ImagePath(imageName),
                        ImagePullPolicy: corev1.PullAlways,
                        Env: []v1.EnvVar{{
                            Name:  "BROKER_URL",
                            Value: url,
                        }},
                    }},
                    RestartPolicy: corev1.RestartPolicyNever,
                },
            },
        },
    }
}
