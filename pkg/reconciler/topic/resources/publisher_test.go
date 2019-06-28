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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis/pubsub/v1alpha1"
)

func TestMakePublisher(t *testing.T) {
	topic := &v1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "topic-name",
			Namespace: "topic-namespace",
		},
		Spec: v1alpha1.TopicSpec{
			ServiceAccountName: "topic-svc-acct",
			Project:            "eventing-name",
			Topic:              "topic-name",
			Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "eventing-secret-name",
				},
				Key: "eventing-secret-key",
			},
		},
	}

	pub := MakePublisher(&PublisherArgs{
		Image:  "test-image",
		Topic:  topic,
		Labels: GetLabels("controller-name", "topic-name"),
	})

	gotb, _ := json.MarshalIndent(pub, "", "  ")
	got := string(gotb)

	want := `{
  "metadata": {
    "generateName": "pubsub-publisher-topic-name-",
    "namespace": "topic-namespace",
    "creationTimestamp": null,
    "labels": {
      "pubsub.cloud.run/controller": "controller-name",
      "pubsub.cloud.run/topic": "topic-name"
    },
    "ownerReferences": [
      {
        "apiVersion": "pubsub.cloud.run/v1alpha1",
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
          "pubsub.cloud.run/controller": "controller-name",
          "pubsub.cloud.run/topic": "topic-name"
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
            "name": "publisher",
            "image": "test-image",
            "env": [
              {
                "name": "GOOGLE_APPLICATION_CREDENTIALS",
                "value": "/var/secrets/google/eventing-secret-key"
              },
              {
                "name": "PROJECT_ID",
                "value": "eventing-name"
              },
              {
                "name": "PUBSUB_TOPIC_ID",
                "value": "topic-name"
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
        ],
        "serviceAccountName": "topic-svc-acct"
      }
    }
  },
  "status": {}
}`

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakePublisherSelector(t *testing.T) {
	selector := GetLabelSelector("controller-name", "topic-name")

	want := "pubsub.cloud.run/controller=controller-name,pubsub.cloud.run/topic=topic-name"

	got := selector.String()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected selector (-want, +got) = %v", diff)
	}
}
