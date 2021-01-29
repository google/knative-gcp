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
	"k8s.io/apimachinery/pkg/fields"
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
				t.Errorf("Diffenrent Podlist length, want %v, got %v ", len(tc.wantNames), len(pl.Items))
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

func TestGetEventList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		objects       []runtime.Object
		fieldSelector fields.Selector
		wantNames     []string
	}{
		{
			name: "using correct field selector to get eventlist",
			objects: []runtime.Object{
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: testNS,
					},
					InvolvedObject: corev1.ObjectReference{
						Kind: "Pod",
						Name: "pod-1",
					},
					Type: "Warning",
				},
			},
			fieldSelector: podWarningFieldSelector("pod-1"),
			wantNames: []string{
				"event-1",
			},
		},
		{
			name: "using incorrect field selector to get eventlist",
			objects: []runtime.Object{
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: testNS,
					},
					InvolvedObject: corev1.ObjectReference{
						Kind: "Non-Pod",
						Name: "pod-1",
					},
					Type: "Warning",
				},
			},
			fieldSelector: podWarningFieldSelector("Pod"),
			wantNames:     []string{},
		},
		{
			name: "using incorrect namespace to get eventlist",
			objects: []runtime.Object{
				&corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: "fake-namespace",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind: "Non-Pod",
						Name: "pod-1",
					},
					Type: "Warning",
				},
			},
			fieldSelector: podWarningFieldSelector("Pod"),
			wantNames:     []string{},
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

			el, _ := GetEventList(ctx, cs, "pod-1", testNS)

			// Field Selector doesn't apply in fake clients List. We need to filter it manually.
			// More information: https://github.com/kubernetes/kubernetes/issues/78824.
			el = filterField(tc.fieldSelector, el)

			for _, event := range el.Items {
				found := false
				for _, wantName := range tc.wantNames {
					if event.Name == wantName {
						found = true
					}
					if !found {
						t.Error("Unexpected event", event.Name)
					}
				}
			}
			if len(el.Items) != len(tc.wantNames) {
				t.Errorf("Diffenrent Eventlist length, want %v, got %v ", len(tc.wantNames), len(el.Items))
			}
		})
	}
}

func TestGetMountFailureMessageFromEventList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		eventlist   *corev1.EventList
		secret      *corev1.SecretKeySelector
		wantMessage string
	}{
		{
			name: "qualified secret related message from event",
			eventlist: &corev1.EventList{
				Items: []corev1.Event{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: testNS,
					},
					Message: `secret "google-cloud-key" not found`,
				}},
			},
			secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			wantMessage: `secret "google-cloud-key" not found`,
		},
		{
			name: "qualified key related message from event",
			eventlist: &corev1.EventList{
				Items: []corev1.Event{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: testNS,
					},
					Message: "couldn't find key key.json in Secret test/google-cloud-key",
				}},
			},
			secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			wantMessage: "couldn't find key key.json in Secret test/google-cloud-key",
		},
		{
			name: "un-qualified message",
			eventlist: &corev1.EventList{
				Items: []corev1.Event{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "event-1",
						Namespace: testNS,
					},
					Message: "non",
				}},
			},
			secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "google-cloud-key"},
				Key:                  "key.json",
			},
			wantMessage: "",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getMessage := GetMountFailureMessageFromEventList(tc.eventlist, tc.secret)
			if diff := cmp.Diff(tc.wantMessage, getMessage); diff != "" {
				t.Error("unexpected termination message (-want, +got) = ", diff)
			}
		})
	}
}

func filterField(selector fields.Selector, eventList *corev1.EventList) *corev1.EventList {
	list := &corev1.EventList{}
	for _, event := range eventList.Items {
		if selector.Matches(eventSet(event)) {
			list.Items = append(list.Items, event)
		}
	}
	return list
}

func eventSet(event corev1.Event) fields.Set {
	return map[string]string{
		"involvedObject.kind": event.InvolvedObject.Kind,
		"type":                event.Type,
		"involvedObject.name": event.InvolvedObject.Name,
	}
}
