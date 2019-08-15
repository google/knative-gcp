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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

func TestMakePublisherV1alpha1(t *testing.T) {
	decorator := &v1alpha1.Decorator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dec-name",
			Namespace: "dec-namespace",
		},
		Spec: v1alpha1.DecoratorSpec{
			Extensions: map[string]string{
				"foo":   "bar",
				"boosh": "kakow",
			},
		},
		Status: v1alpha1.DecoratorStatus{
			SinkURI: "http://sinkUri",
		},
	}

	pub := MakeDecoratorV1alpha1(&DecoratorArgs{
		Image:     "test-image",
		Decorator: decorator,
		Labels:    GetLabels("controller-name"),
	})

	gotb, _ := json.MarshalIndent(pub, "", "  ")
	got := string(gotb)

	want := `{
  "metadata": {
    "name": "cre-dec-name-decorator",
    "namespace": "dec-namespace",
    "creationTimestamp": null,
    "labels": {
      "messaging.cloud.run/controller": "controller-name"
    },
    "ownerReferences": [
      {
        "apiVersion": "messaging.cloud.run/v1alpha1",
        "kind": "Decorator",
        "name": "dec-name",
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
          "messaging.cloud.run/controller": "controller-name"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "publisher",
            "image": "test-image",
            "env": [
              {
                "name": "K_CE_EXTENSIONS",
                "value": "eyJib29zaCI6Imtha293IiwiZm9vIjoiYmFyIn0="
              },
              {
                "name": "K_SINK",
                "value": "http://sinkUri"
              }
            ],
            "resources": {}
          }
        ]
      }
    }
  },
  "status": {}
}`

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
		t.Log(got)
	}
}

func TestAdditions(t *testing.T) {
	additions := map[string]string{
		"foo":   "bar",
		"boosh": "kakow",
	}
	additionsString := MakeDecoratorExtensionsString(additions)
	// Test the to string
	{
		want := "eyJib29zaCI6Imtha293IiwiZm9vIjoiYmFyIn0="
		got := additionsString
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
			t.Log(got)
		}
	}
	// Test the to map
	{
		want := additions
		got := MakeDecoratorExtensionsMap(additionsString)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
			t.Log(got)
		}
	}
}
