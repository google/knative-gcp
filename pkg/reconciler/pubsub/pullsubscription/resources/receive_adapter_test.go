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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestMakeMinimumReceiveAdapter(t *testing.T) {
	src := &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: "source-namespace",
		},
		Spec: v1alpha1.PullSubscriptionSpec{
			Project: "eventing-name",
			Topic:   "topic",
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
	}

	got := MakeReceiveAdapter(context.Background(), &ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        "sink-uri",
		LoggingConfig:  "LoggingConfig-ABC123",
		MetricsConfig:  "MetricsConfig-ABC123",
		TracingConfig:  "TracingConfig-ABC123",
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "source-namespace",
			Name:        "cre-pull-",
			Annotations: nil,
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "pubsub.cloud.google.com/v1alpha1",
				Kind:               "PullSubscription",
				Name:               "source-name",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
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
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "receive-adapter",
						Image: "test-image",
						Env: []corev1.EnvVar{{
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "/var/secrets/google/eventing-secret-key",
						}, {
							Name:      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
							ValueFrom: &corev1.EnvVarSource{SecretKeyRef: src.Spec.Secret},
						}, {
							Name:  "PROJECT_ID",
							Value: "eventing-name",
						}, {
							Name:  "PUBSUB_TOPIC_ID",
							Value: "topic",
						}, {
							Name:  "PUBSUB_SUBSCRIPTION_ID",
							Value: "sub-id",
						}, {
							Name:  "SINK_URI",
							Value: "sink-uri",
						}, {
							Name: "TRANSFORMER_URI",
						}, {
							Name: "ADAPTER_TYPE",
						}, {
							Name:  "SEND_MODE",
							Value: "binary",
						}, {
							Name: "K_CE_EXTENSIONS",
						}, {
							Name:  "K_METRICS_CONFIG",
							Value: "MetricsConfig-ABC123",
						}, {
							Name:  "K_LOGGING_CONFIG",
							Value: "LoggingConfig-ABC123",
						}, {
							Name:  "K_TRACING_CONFIG",
							Value: "TracingConfig-ABC123",
						}, {
							Name:  "NAME",
							Value: "source-name",
						}, {
							Name:  "NAMESPACE",
							Value: "source-namespace",
						}, {
							Name:  "RESOURCE_GROUP",
							Value: defaultResourceGroup,
						}, {
							Name:  "METRICS_DOMAIN",
							Value: metricsDomain,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      credsVolume,
							MountPath: credsMountPath,
						}},
						Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 9090}},
					}},
					Volumes: []corev1.Volume{{
						Name: credsVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "eventing-secret-name",
							},
						},
					}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakeFullReceiveAdapter(t *testing.T) {
	src := &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: "source-namespace",
			Annotations: map[string]string{
				"metrics-resource-group": "test-resource-group",
			},
		},
		Spec: v1alpha1.PullSubscriptionSpec{
			Project:     "eventing-name",
			Topic:       "topic",
			AdapterType: "adapter-type",
			SourceSpec: duckv1.SourceSpec{
				CloudEventOverrides: &duckv1.CloudEventOverrides{
					Extensions: map[string]string{
						"foo": "bar", // base64 value is eyJmb28iOiJiYXIifQ==
					},
				},
			},
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
	}

	got := MakeReceiveAdapter(context.Background(), &ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        "sink-uri",
		TransformerURI: "transformer-uri",
		LoggingConfig:  "LoggingConfig-ABC123",
		MetricsConfig:  "MetricsConfig-ABC123",
		TracingConfig:  "TracingConfig-ABC123",
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "source-namespace",
			Name:        "cre-pull-",
			Annotations: src.Annotations,
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "pubsub.cloud.google.com/v1alpha1",
				Kind:               "PullSubscription",
				Name:               "source-name",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			}},
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
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "receive-adapter",
						Image: "test-image",
						Env: []corev1.EnvVar{{
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "/var/secrets/google/eventing-secret-key",
						}, {
							Name:      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
							ValueFrom: &corev1.EnvVarSource{SecretKeyRef: src.Spec.Secret},
						}, {
							Name:  "PROJECT_ID",
							Value: "eventing-name",
						}, {
							Name:  "PUBSUB_TOPIC_ID",
							Value: "topic",
						}, {
							Name:  "PUBSUB_SUBSCRIPTION_ID",
							Value: "sub-id",
						}, {
							Name:  "SINK_URI",
							Value: "sink-uri",
						}, {
							Name:  "TRANSFORMER_URI",
							Value: "transformer-uri",
						}, {
							Name:  "ADAPTER_TYPE",
							Value: "adapter-type",
						}, {
							Name:  "SEND_MODE",
							Value: "binary",
						}, {
							Name:  "K_CE_EXTENSIONS",
							Value: "eyJmb28iOiJiYXIifQ==",
						}, {
							Name:  "K_METRICS_CONFIG",
							Value: "MetricsConfig-ABC123",
						}, {
							Name:  "K_LOGGING_CONFIG",
							Value: "LoggingConfig-ABC123",
						}, {
							Name:  "K_TRACING_CONFIG",
							Value: "TracingConfig-ABC123",
						}, {
							Name:  "NAME",
							Value: "source-name",
						}, {
							Name:  "NAMESPACE",
							Value: "source-namespace",
						}, {
							Name:  "RESOURCE_GROUP",
							Value: "test-resource-group",
						}, {
							Name:  "METRICS_DOMAIN",
							Value: metricsDomain,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      credsVolume,
							MountPath: credsMountPath,
						}},
						Ports: []corev1.ContainerPort{{
							Name:          "metrics",
							ContainerPort: 9090,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: credsVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "eventing-secret-name",
							},
						},
					}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
