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
	"knative.dev/pkg/kmeta"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

var (
	gServiceAccountName = "test@test.iam.gserviceaccount.com"
	kServiceAccountName = "test"
)

func TestGenerateServiceAccountName(t *testing.T) {
	want := kServiceAccountName
	got := GenerateServiceAccountName(gServiceAccountName)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestMakeServiceAccount(t *testing.T) {
	want := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      kServiceAccountName,
			Annotations: map[string]string{
				"iam.gke.io/gcp-service-account": gServiceAccountName,
			},
		},
	}
	got := MakeServiceAccount("default", gServiceAccountName)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestOwnerReferenceExists(t *testing.T) {
	source := &v1alpha1.CloudSchedulerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler-name",
			Namespace: "scheduler-namespace",
			UID:       "scheduler-uid",
		},
		Spec: v1alpha1.CloudSchedulerSourceSpec{
			PubSubSpec: duckv1alpha1.PubSubSpec{
				Project: "project-123",
				Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "eventing-secret-name",
					},
					Key: "eventing-secret-key",
				},
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "Kitchen",
							Name:       "sink",
						},
					},
				},
			},
		},
	}
	kServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      kServiceAccountName,
			Annotations: map[string]string{
				"iam.gke.io/gcp-service-account": gServiceAccountName,
			},
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(source)},
		},
	}
	ownerReference := metav1.OwnerReference{
		APIVersion:         source.APIVersion,
		Kind:               source.Kind,
		Name:               source.Name,
		UID:                "",
		Controller:         nil,
		BlockOwnerDeletion: nil,
	}

	want := true
	got := OwnerReferenceExists(kServiceAccount, ownerReference)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}
