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

package resources

import (
	"testing"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/events/v1alpha1"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeReceiveAdapter(t *testing.T) {
	src := &v1alpha1.PubSubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: "source-namespace",
		},
		Spec: v1alpha1.PubSubSourceSpec{
			ServiceAccountName: "source-svc-acct",
			GoogleCloudProject: "eventing-name",
			Topic:              "topic",
			GcpCredsSecret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
	}

	got := MakeReceiveAdapter(&ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        "sink-uri",
	})

	one := int32(1)
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "source-namespace",
			GenerateName: "pubsub-source-name-",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "source-svc-acct",
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: "test-image",
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: "/var/secrets/google/eventing-secret-key",
								},
								{
									Name:  "GCPPUBSUB_PROJECT",
									Value: "eventing-name",
								},
								{
									Name:  "GCPPUBSUB_TOPIC",
									Value: "topic",
								},
								{
									Name:  "GCPPUBSUB_SUBSCRIPTION_ID",
									Value: "sub-id",
								},
								{
									Name:  "SINK_URI",
									Value: "sink-uri",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      credsVolume,
									MountPath: credsMountPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: credsVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "eventing-secret-name",
								},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
