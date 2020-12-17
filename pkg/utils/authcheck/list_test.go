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

package authcheck

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	fakeKubeClient "k8s.io/client-go/kubernetes/fake"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

func TestGetPodList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		objects   []runtime.Object
		labels    labels.Selector
		wantNames []string
	}{
		{
			name: "using correct labels to get podlist",
			objects: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testNS,
						Labels: map[string]string{
							"label1": "val1",
							"label2": "val2",
						},
					},
				},
			},
			labels: labels.SelectorFromSet(map[string]string{
				"label1": "val1",
				"label2": "val2",
			}),
			wantNames: []string{
				"pod-1",
			},
		},
		{
			name: "using incorrect labels to get podlist",
			objects: []runtime.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testNS,
						Labels: map[string]string{
							"label2": "val2",
						},
					},
				},
			},
			labels: labels.SelectorFromSet(map[string]string{
				"label1": "val1",
				"label2": "val2",
			}),
			wantNames: []string{},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			//  Sets up the the Context and the fake informers for the tests.
			ctx, cancel, _ := pkgtesting.SetupFakeContextWithCancel(t)
			defer cancel()

			// Form a fake Kube Client from tc.objects.
			cs := fakeKubeClient.NewSimpleClientset(tc.objects...)

			pl, _ := GetPodList(ctx, tc.labels, cs, testNS)

			for _, pod := range pl.Items {
				found := false
				for _, wantName := range tc.wantNames {
					if pod.Name == wantName {
						found = true
					}
					if !found {
						t.Error("Unexpected pod", pod.Name)
					}
				}
			}
			if len(pl.Items) != len(tc.wantNames) {
				t.Errorf("Diffenrent Podlist length, want %v, got %v ", len(pl.Items), len(tc.wantNames))
			}
		})
	}
}

func TestGetTerminationLogFromPodList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		podlist     *corev1.PodList
		wantMessage string
	}{
		{
			name: "qualified message in Pod State",
			podlist: &corev1.PodList{
				Items: []corev1.Pod{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testNS,
						Labels: map[string]string{
							"label1": "val1",
							"label2": "val2",
						},
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Message: "checking authentication",
								},
							}}},
					},
				}},
			},
			wantMessage: "checking authentication",
		},
		{
			name: "qualified message in Pod LastTerminationState",
			podlist: &corev1.PodList{
				Items: []corev1.Pod{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testNS,
						Labels: map[string]string{
							"label1": "val1",
							"label2": "val2",
						},
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Message: "checking authentication",
								},
							}}},
					}},
				},
			},
			wantMessage: "checking authentication",
		},
		{
			name: "un-qualified message",
			podlist: &corev1.PodList{
				Items: []corev1.Pod{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: testNS,
						Labels: map[string]string{
							"label1": "val1",
							"label2": "val2",
						},
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{{
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Message: "non-checking",
								},
							}}},
					},
				}},
			},
			wantMessage: "",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getMessage := GetTerminationLogFromPodList(tc.podlist)
			if diff := cmp.Diff(getMessage, tc.wantMessage); diff != "" {
				t.Error("unexpected termination message (-want, +got) = ", diff)
			}
		})
	}
}
