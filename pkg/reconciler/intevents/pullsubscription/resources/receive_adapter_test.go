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
	"github.com/google/knative-gcp/pkg/apis/duck"
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	"github.com/google/knative-gcp/pkg/apis/intevents"
	intereventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	testingmetadata "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestMakeMinimumReceiveAdapter(t *testing.T) {
	ps := &intereventsv1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testname",
			Namespace: "testnamespace",
			Labels: map[string]string{
				intevents.SourceLabelKey: "my-source-name",
			},
		},
		Spec: intereventsv1.PullSubscriptionSpec{
			PubSubSpec: gcpduckv1.PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "eventing-secret-name",
					},
					Key: "eventing-secret-key",
				},
				Project: "eventing-name",
			},
			Topic:       "topic",
			AdapterType: "source-adapter-type",
		},
	}

	got := MakeReceiveAdapter(context.Background(), &ReceiveAdapterArgs{
		Image:            "test-image",
		PullSubscription: ps,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        apis.HTTP("sink-uri"),
		LoggingConfig:  "LoggingConfig-ABC123",
		MetricsConfig:  "MetricsConfig-ABC123",
		TracingConfig:  "TracingConfig-ABC123",
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "testnamespace",
			Name:        "cre-src-testname-",
			Annotations: nil,
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "internal.events.cloud.google.com/v1",
				Kind:               "PullSubscription",
				Name:               "testname",
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
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("600Mi"),
								corev1.ResourceCPU:    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
								corev1.ResourceCPU:    resource.MustParse("400m"),
							},
						},
						Env: []corev1.EnvVar{{
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
							Value: "http://sink-uri",
						}, {
							Name: "TRANSFORMER_URI",
						}, {
							Name:  "ADAPTER_TYPE",
							Value: "source-adapter-type",
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
							Value: "testname",
						}, {
							Name:  "NAMESPACE",
							Value: "testnamespace",
						}, {
							Name:  "RESOURCE_GROUP",
							Value: defaultResourceGroup,
						}, {
							Name:  "METRICS_DOMAIN",
							Value: metricsDomain,
						}, {
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "/var/secrets/google/eventing-secret-key",
						}, {
							Name:      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
							ValueFrom: &corev1.EnvVarSource{SecretKeyRef: ps.Spec.Secret},
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
	ps := &intereventsv1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testname",
			Namespace: "testnamespace",
			Annotations: map[string]string{
				"metrics-resource-group": "test-resource-group",
			},
		},
		Spec: intereventsv1.PullSubscriptionSpec{
			PubSubSpec: gcpduckv1.PubSubSpec{
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "eventing-secret-name",
					},
					Key: "eventing-secret-key",
				},
				Project: "eventing-name",
				SourceSpec: duckv1.SourceSpec{
					CloudEventOverrides: &duckv1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar", // base64 value is eyJmb28iOiJiYXIifQ==
						},
					},
				},
			},
			Topic:       "topic",
			AdapterType: string(converters.PubSubPull),
		},
	}

	got := MakeReceiveAdapter(context.Background(), &ReceiveAdapterArgs{
		Image:            "test-image",
		PullSubscription: ps,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        apis.HTTP("sink-uri"),
		TransformerURI: apis.HTTP("transformer-uri"),
		LoggingConfig:  "LoggingConfig-ABC123",
		MetricsConfig:  "MetricsConfig-ABC123",
		TracingConfig:  "TracingConfig-ABC123",
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "testnamespace",
			Name:        "cre-ps-testname-",
			Annotations: ps.Annotations,
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "internal.events.cloud.google.com/v1",
				Kind:               "PullSubscription",
				Name:               "testname",
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
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("600Mi"),
								corev1.ResourceCPU:    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
								corev1.ResourceCPU:    resource.MustParse("400m"),
							},
						},
						Env: []corev1.EnvVar{{
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
							Value: "http://sink-uri",
						}, {
							Name:  "TRANSFORMER_URI",
							Value: "http://transformer-uri",
						}, {
							Name:  "ADAPTER_TYPE",
							Value: string(converters.PubSubPull),
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
							Value: "testname",
						}, {
							Name:  "NAMESPACE",
							Value: "testnamespace",
						}, {
							Name:  "RESOURCE_GROUP",
							Value: "test-resource-group",
						}, {
							Name:  "METRICS_DOMAIN",
							Value: metricsDomain,
						}, {
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "/var/secrets/google/eventing-secret-key",
						}, {
							Name:      "GOOGLE_APPLICATION_CREDENTIALS_JSON",
							ValueFrom: &corev1.EnvVarSource{SecretKeyRef: ps.Spec.Secret},
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

func TestMakeReceiveAdapterWithServiceAccount(t *testing.T) {
	serviceAccountName := "test"
	ps := &intereventsv1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testname",
			Namespace: "testnamespace",
			Annotations: map[string]string{
				"metrics-resource-group":   "test-resource-group",
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			},
		},
		Spec: intereventsv1.PullSubscriptionSpec{
			PubSubSpec: gcpduckv1.PubSubSpec{
				IdentitySpec: gcpduckv1.IdentitySpec{
					ServiceAccountName: serviceAccountName,
				},
				Project: "eventing-name",
				SourceSpec: duckv1.SourceSpec{
					CloudEventOverrides: &duckv1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar", // base64 value is eyJmb28iOiJiYXIifQ==
						},
					},
				},
			},
			Topic:       "topic",
			AdapterType: "adapter-type",
		},
	}

	got := MakeReceiveAdapter(context.Background(), &ReceiveAdapterArgs{
		Image:            "test-image",
		PullSubscription: ps,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SubscriptionID: "sub-id",
		SinkURI:        apis.HTTP("sink-uri"),
		TransformerURI: apis.HTTP("transformer-uri"),
		LoggingConfig:  "LoggingConfig-ABC123",
		MetricsConfig:  "MetricsConfig-ABC123",
		TracingConfig:  "TracingConfig-ABC123",
	})

	one := int32(1)
	yes := true
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "testnamespace",
			Name:        "cre-ps-testname-",
			Annotations: ps.Annotations,
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "internal.events.cloud.google.com/v1",
				Kind:               "PullSubscription",
				Name:               "testname",
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
					ServiceAccountName: "test",
					Containers: []corev1.Container{{
						Name:  "receive-adapter",
						Image: "test-image",
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("600Mi"),
								corev1.ResourceCPU:    resource.MustParse("500m"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
								corev1.ResourceCPU:    resource.MustParse("400m"),
							},
						},
						Env: []corev1.EnvVar{{
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
							Value: "http://sink-uri",
						}, {
							Name:  "TRANSFORMER_URI",
							Value: "http://transformer-uri",
						}, {
							Name:  "ADAPTER_TYPE",
							Value: string(converters.PubSubPull),
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
							Value: "testname",
						}, {
							Name:  "NAMESPACE",
							Value: "testnamespace",
						}, {
							Name:  "RESOURCE_GROUP",
							Value: "test-resource-group",
						}, {
							Name:  "METRICS_DOMAIN",
							Value: metricsDomain,
						}},
						Ports: []corev1.ContainerPort{{
							Name:          "metrics",
							ContainerPort: 9090,
						}},
					}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
