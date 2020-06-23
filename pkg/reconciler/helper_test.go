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

package reconciler

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	pkgreconcilertesting "knative.dev/pkg/reconciler/testing"
)

const (
	deploymentCreatedEvent = "Normal DeploymentCreated Created deployment testns/test"
	deploymentUpdatedEvent = "Normal DeploymentUpdated Updated deployment testns/test"
	serviceCreatedEvent    = "Normal ServiceCreated Created service testns/test"
	serviceUpdatedEvent    = "Normal ServiceUpdated Updated service testns/test"
	configmapCreatedEvent  = "Normal ConfigMapCreated Created configmap testns/test"
	configmapUpdatedEvent  = "Normal ConfigMapUpdated Updated configmap testns/test"
)

var (
	// obj can be anything that implements runtime.Object. In real reconcilers this should be the object being reconciled.
	obj = &corev1.Namespace{}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       appsv1.DeploymentSpec{MinReadySeconds: 10},
	}
	deploymentDifferentSpec = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       appsv1.DeploymentSpec{MinReadySeconds: 20},
	}

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

	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Data:       map[string]string{"data": "value"},
		BinaryData: map[string][]byte{"binary": []byte{'b'}},
	}
	cmDifferentData = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Data:       map[string]string{"data": "different value"},
		BinaryData: map[string][]byte{"binary": {'b'}},
	}
	cmDifferentBinaryData = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Data:       map[string]string{"data": "different value"},
		BinaryData: map[string][]byte{"binary": {'d'}},
	}

	deploymentCreateFailure = pkgreconcilertesting.InduceFailure("create", "deployments")
	deploymentUpdateFailure = pkgreconcilertesting.InduceFailure("update", "deployments")
	serviceCreateFailure    = pkgreconcilertesting.InduceFailure("create", "services")
	serviceUpdateFailure    = pkgreconcilertesting.InduceFailure("update", "services")
	configmapCreateFailure  = pkgreconcilertesting.InduceFailure("create", "configmaps")
	configmapUpdateFailure  = pkgreconcilertesting.InduceFailure("update", "configmaps")

	tr = &testRunner{}
)

type commonCase struct {
	name string
	// objects to be injected in KubeClientSet and informers before running the test.
	existing   []runtime.Object
	reactions  []clientgotesting.ReactionFunc
	wantEvents []string
	wantErr    bool
}

func TestDeploymentReconciler(t *testing.T) {
	var tests = []struct {
		commonCase
		in   *appsv1.Deployment
		want *appsv1.Deployment
	}{
		{
			commonCase: commonCase{
				name:     "deployment exists, nothing to do",
				existing: []runtime.Object{deployment},
			},
			in:   deployment,
			want: deployment,
		},
		{
			commonCase: commonCase{
				name:       "deployment created",
				wantEvents: []string{deploymentCreatedEvent},
			},
			in:   deployment,
			want: deployment,
		},
		{
			commonCase: commonCase{
				name:      "deployment creation error",
				reactions: []clientgotesting.ReactionFunc{deploymentCreateFailure},
				wantErr:   true,
			},
			in: deployment,
		},
		{
			commonCase: commonCase{
				name:       "deployment updated",
				existing:   []runtime.Object{deploymentDifferentSpec},
				wantEvents: []string{deploymentUpdatedEvent},
			},
			in:   deployment,
			want: deployment,
		},
		{
			commonCase: commonCase{
				name:      "deployment update error",
				reactions: []clientgotesting.ReactionFunc{deploymentUpdateFailure},
				existing:  []runtime.Object{deploymentDifferentSpec},
				wantErr:   true,
			},
			in: deployment,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr.setup(test.commonCase)

			rec := DeploymentReconciler{
				KubeClient: tr.client,
				Lister:     tr.listers.GetDeploymentLister(),
				Recorder:   tr.recorder,
			}
			out, err := rec.ReconcileDeployment(obj, test.in)

			tr.verify(t, test.commonCase, err)

			if diff := cmp.Diff(out, test.want); diff != "" {
				t.Errorf("Unexpected reconciler result (-got, +want): %s", diff)
				return
			}
		})
	}
}

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
			out, err := rec.ReconcileService(obj, test.in)

			tr.verify(t, test.commonCase, err)

			if diff := cmp.Diff(out, test.want); diff != "" {
				t.Errorf("Unexpected reconciler result (-got, +want): %s", diff)
				return
			}
		})
	}
}

func TestConfigMapReconciler(t *testing.T) {
	var tests = []struct {
		commonCase
		in   *corev1.ConfigMap
		want *corev1.ConfigMap
	}{
		{
			commonCase: commonCase{
				name:     "configmap exists, nothing to do",
				existing: []runtime.Object{cm},
			},
			in:   cm,
			want: cm,
		},
		{
			commonCase: commonCase{
				name:       "cofigmap created",
				wantEvents: []string{configmapCreatedEvent},
			},
			in:   cm,
			want: cm,
		},
		{
			commonCase: commonCase{
				name:      "configmap creation error",
				reactions: []clientgotesting.ReactionFunc{configmapCreateFailure},
				wantErr:   true,
			},
			in: cm,
		},
		{
			commonCase: commonCase{
				name:       "cofigmap updated - different data",
				existing:   []runtime.Object{cmDifferentData},
				wantEvents: []string{configmapUpdatedEvent},
			},
			in:   cm,
			want: cm,
		},
		{
			commonCase: commonCase{
				name:       "cofigmap updated - different binary data",
				existing:   []runtime.Object{cmDifferentBinaryData},
				wantEvents: []string{configmapUpdatedEvent},
			},
			in:   cm,
			want: cm,
		},
		{
			commonCase: commonCase{
				name:      "configmap update error",
				reactions: []clientgotesting.ReactionFunc{configmapUpdateFailure},
				existing:  []runtime.Object{cmDifferentData},
				wantErr:   true,
			},
			in: cm,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tr.setup(test.commonCase)

			rec := ConfigMapReconciler{
				KubeClient: tr.client,
				Lister:     tr.listers.GetConfigMapLister(),
				Recorder:   tr.recorder,
			}
			out, err := rec.ReconcileConfigMap(obj, test.in)

			tr.verify(t, test.commonCase, err)

			if diff := cmp.Diff(out, test.want); diff != "" {
				t.Errorf("Unexpected reconciler result (-got, +want): %s", diff)
				return
			}
		})
	}
}

// testRunner helps to setup resources such as fake KubeClientSet and informers, as well as verify the common test case.
type testRunner struct {
	client   *fake.Clientset
	listers  reconcilertesting.Listers
	recorder *record.FakeRecorder
}

func (r *testRunner) setup(cc commonCase) {
	objs := append(cc.existing)
	r.client = fake.NewSimpleClientset(objs...)
	for _, reaction := range cc.reactions {
		r.client.PrependReactor("*", "*", reaction)
	}
	r.recorder = record.NewFakeRecorder(len(cc.wantEvents))
	r.listers = reconcilertesting.NewListers(objs)
}

func (r *testRunner) verify(t *testing.T, cc commonCase, err error) {
	if !cc.wantErr && err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cc.wantErr && err == nil {
		t.Fatalf("Expect error but got nil")
	}

	for _, event := range cc.wantEvents {
		got := <-r.recorder.Events
		if got != event {
			t.Errorf("Unexpected event recorded, got: %v, want: %v", got, event)
		}
	}
}
