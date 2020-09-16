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

package volume

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testNS  = "test-ns"
	otherNS = "some-other-ns"
)

var ls = map[string]string{"key": "value"}

func pod(ns, name string, labels map[string]string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func TestUpdateAnnotation(t *testing.T) {
	var (
		annotations = map[string]string{
			"somekey":           "somevalue",
			volumeGenerationKey: "1",
		}
		updatedAnnotations = map[string]string{
			"somekey":           "somevalue",
			volumeGenerationKey: "2",
		}
	)
	var tests = []struct {
		name     string
		pods     []runtime.Object
		wantPods []corev1.Pod
		wantErr  bool
	}{
		{
			name: "no pods",
		},
		{
			name: "multiple pods, one matches both selector and namespace, one matches selector but in a different namespace, one doesn't match selector",
			pods: []runtime.Object{
				pod(testNS, "sameNS1", ls, annotations),
				pod(testNS, "sameNS2", map[string]string{"someKey": "someValue"}, annotations),
				pod(otherNS, "differentNS", ls, annotations),
			},
			wantPods: []corev1.Pod{
				*pod(testNS, "sameNS1", ls, updatedAnnotations),
				*pod(testNS, "sameNS2", map[string]string{"someKey": "someValue"}, annotations),
				*pod(otherNS, "differentNS", ls, annotations),
			},
		},
		{
			name: "multiple matching pods, one with nil annotation, one with empty annotation, one with annotation",
			pods: []runtime.Object{
				pod(testNS, "pod1", ls, nil),
				pod(testNS, "pod2", ls, make(map[string]string)),
				pod(testNS, "pod3", ls, annotations),
			},
			wantPods: []corev1.Pod{
				*pod(testNS, "pod1", ls, map[string]string{volumeGenerationKey: "1"}),
				*pod(testNS, "pod2", ls, map[string]string{volumeGenerationKey: "1"}),
				*pod(testNS, "pod3", ls, updatedAnnotations),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.pods...)
			listers := reconcilertesting.NewListers(test.pods)

			err := UpdateVolumeGeneration(context.Background(), client, listers.GetPodLister(), testNS, ls)
			if !test.wantErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if test.wantErr && err == nil {
				t.Fatalf("Expect error but got nil")
			}

			gotPods := getPods(t, client, testNS, otherNS)
			if len(gotPods) != len(test.wantPods) {
				t.Fatalf("Number of got pods is unexpected, got:%v, want:%v", len(gotPods), len(test.wantPods))
			}
			for i := range test.wantPods {
				if diff := cmp.Diff(gotPods[i], test.wantPods[i]); diff != "" {
					t.Errorf("Pod %v differ (-got, +want): %s", i, diff)
					return
				}
			}
		})
	}
}

// getPods returns pods from all given namespaces.
func getPods(t *testing.T, client kubernetes.Interface, namespaces ...string) []corev1.Pod {
	var pods []corev1.Pod
	for _, namespace := range namespaces {
		gotPods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Failed to get pods for namespace %v: %v", namespace, err)
		}
		pods = append(pods, gotPods.Items...)
	}
	return pods
}
