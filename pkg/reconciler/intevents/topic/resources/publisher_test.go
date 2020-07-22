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
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/duck"
	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	testingmetadata "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
)

func TestMakePublisher(t *testing.T) {
	topic := &v1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "topic-name",
			Namespace: "topic-namespace",
		},
		Spec: v1.TopicSpec{
			Project: "eventing-name",
			Topic:   "topic-name",
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
	}

	pub := MakePublisher(&PublisherArgs{
		Image:         "test-image",
		Topic:         topic,
		Labels:        GetLabels("controller-name", "topic-name"),
		TracingConfig: "TracingConfig-ABC123",
	})

	gotb, _ := json.MarshalIndent(pub, "", "  ")
	got := string(gotb)

	want := `{
  "metadata": {
    "name": "cre-topic-name-publish",
    "namespace": "topic-namespace",
    "creationTimestamp": null,
    "labels": {
      "internal.events.cloud.google.com/controller": "controller-name",
      "internal.events.cloud.google.com/topic": "topic-name"
    },
    "ownerReferences": [
      {
        "apiVersion": "internal.events.cloud.google.com/v1",
        "kind": "Topic",
        "name": "topic-name",
        "uid": "",
        "controller": true,
        "blockOwnerDeletion": true
      }
    ]
  },
  "spec": {
    "template": {
      "metadata": {
        "creationTimestamp": null,
        "labels": {
          "internal.events.cloud.google.com/controller": "controller-name",
          "internal.events.cloud.google.com/topic": "topic-name"
        }
      },
      "spec": {
        "volumes": [
          {
            "name": "google-cloud-key",
            "secret": {
              "secretName": "eventing-secret-name"
            }
          }
        ],
        "containers": [
          {
            "name": "",
            "image": "test-image",
            "env": [
              {
                "name": "PROJECT_ID",
                "value": "eventing-name"
              },
              {
                "name": "PUBSUB_TOPIC_ID",
                "value": "topic-name"
              },
              {
                "name": "K_TRACING_CONFIG",
                "value": "TracingConfig-ABC123"
              },
              {
                "name": "GOOGLE_APPLICATION_CREDENTIALS",
                "value": "/var/secrets/google/eventing-secret-key"
              }
            ],
            "resources": {},
            "volumeMounts": [
              {
                "name": "google-cloud-key",
                "mountPath": "/var/secrets/google"
              }
            ]
          }
        ]
      }
    }
  },
  "status": {}
}`

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakePublisherWithServiceAccount(t *testing.T) {
	serviceAccountName := "test"
	topic := &v1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "topic-name",
			Namespace: "topic-namespace",
			Annotations: map[string]string{
				duck.ClusterNameAnnotation: testingmetadata.FakeClusterName,
			},
		},
		Spec: v1.TopicSpec{
			Project: "eventing-name",
			Topic:   "topic-name",
			IdentitySpec: duckv1.IdentitySpec{
				ServiceAccountName: serviceAccountName,
			},
		},
	}

	got := MakePublisher(&PublisherArgs{
		Image:         "test-image",
		Topic:         topic,
		Labels:        GetLabels("controller-name", "topic-name"),
		TracingConfig: "TracingConfig-ABC123",
	})

	yes := true
	want := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cre-topic-name-publish",
			Namespace:         "topic-namespace",
			CreationTimestamp: metav1.Time{},
			Labels: map[string]string{
				"internal.events.cloud.google.com/controller": "controller-name",
				"internal.events.cloud.google.com/topic":      "topic-name",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "internal.events.cloud.google.com/v1",
				Kind:               "Topic",
				Name:               "topic-name",
				Controller:         &yes,
				BlockOwnerDeletion: &yes,
			},
			},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"internal.events.cloud.google.com/controller": "controller-name",
							"internal.events.cloud.google.com/topic":      "topic-name",
						},
					},
					Spec: servingv1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "",
								Image: "test-image",
								Env: []corev1.EnvVar{{
									Name:  "PROJECT_ID",
									Value: "eventing-name",
								}, {
									Name:  "PUBSUB_TOPIC_ID",
									Value: "topic-name",
								}, {
									Name:  "K_TRACING_CONFIG",
									Value: "TracingConfig-ABC123",
								}},
							}},
							ServiceAccountName: "test",
						},
					},
				}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakePublisherSelector(t *testing.T) {
	selector := GetLabelSelector("controller-name", "topic-name")

	want := "internal.events.cloud.google.com/controller=controller-name,internal.events.cloud.google.com/topic=topic-name"

	got := selector.String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected selector (-want, +got) = %v", diff)
	}
}
