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

package utils

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	pkgreconcilertesting "knative.dev/pkg/reconciler/testing"
)

const (
	serviceCreatedEvent = "Normal ServiceCreated Created service testns/test"
	serviceUpdatedEvent = "Normal ServiceUpdated Updated service testns/test"
)

var (
	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}
	serviceDifferentSpec = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
	}
	endPoints = &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
	}

	serviceCreateFailure = pkgreconcilertesting.InduceFailure("create", "services")
	serviceUpdateFailure = pkgreconcilertesting.InduceFailure("update", "services")
)

func TestServiceReconciler(t *testing.T) {
	var tests = []struct {
		commonCase
		in   *corev1.Service
		want *corev1.Endpoints
	}{
		{
			commonCase: commonCase{
				name:     "service exists, nothing to do",
				existing: []runtime.Object{service, endPoints},
			},
			in:   service,
			want: endPoints,
		},
		{
			commonCase: commonCase{
				name:       "service created",
				existing:   []runtime.Object{endPoints},
				wantEvents: []string{serviceCreatedEvent},
			},
			in:   service,
			want: endPoints,
		},
		{
			commonCase: commonCase{
				name:      "service creation error",
				reactions: []clientgotesting.ReactionFunc{serviceCreateFailure},
				wantErr:   true,
			},
			in: service,
		},
		{
			commonCase: commonCase{
				name:       "service updated",
				existing:   []runtime.Object{serviceDifferentSpec, endPoints},
				wantEvents: []string{serviceUpdatedEvent},
			},
			in:   service,
			want: endPoints,
		},
		{
			commonCase: commonCase{
				name:      "service update error",
				reactions: []clientgotesting.ReactionFunc{serviceUpdateFailure},
				existing:  []runtime.Object{serviceDifferentSpec, endPoints},
				wantErr:   true,
			},
			in: service,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr.setup(test.commonCase)

			rec := ServiceReconciler{
				KubeClient:      tr.client,
				ServiceLister:   tr.listers.GetK8sServiceLister(),
				EndpointsLister: tr.listers.GetEndpointsLister(),
				Recorder:        tr.recorder,
			}
			out, err := rec.ReconcileService(context.Background(), obj, test.in)

			tr.verify(t, test.commonCase, err)

			if diff := cmp.Diff(out, test.want); diff != "" {
				t.Errorf("Unexpected reconciler result (-got, +want): %s", diff)
				return
			}
		})
	}
}
