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
	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	pkgreconcilertesting "knative.dev/pkg/reconciler/testing"
)

const (
	configmapCreatedEvent = "Normal ConfigMapCreated Created configmap testns/test"
	configmapUpdatedEvent = "Normal ConfigMapUpdated Updated configmap testns/test"
)

var (
	// obj can be anything that implements runtime.Object. In real reconcilers this should be the object being reconciled.
	obj = &corev1.Namespace{}

	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Data:       map[string]string{"data": "value"},
		BinaryData: map[string][]byte{"binary": {'b'}},
	}
	cmDifferentData = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Data:       map[string]string{"data": "different value"},
		BinaryData: map[string][]byte{"binary": {'b'}},
	}

	configmapCreateFailure = pkgreconcilertesting.InduceFailure("create", "configmaps")
	configmapUpdateFailure = pkgreconcilertesting.InduceFailure("update", "configmaps")

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

func TestConfigMapReconciler(t *testing.T) {
	var tests = []struct {
		commonCase
		in     *corev1.ConfigMap
		want   *corev1.ConfigMap
		eqFunc func(*corev1.ConfigMap, *corev1.ConfigMap) bool
	}{
		{
			commonCase: commonCase{
				name:     "configmap exists, nothing to do",
				existing: []runtime.Object{cm},
			},
			in:     cm,
			want:   cm,
			eqFunc: DefaultConfigMapEqual,
		},
		{
			commonCase: commonCase{
				name:       "configmap created",
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
			in:     cm,
			eqFunc: DefaultConfigMapEqual,
		},
		{
			commonCase: commonCase{
				name:       "configmap updated - different data",
				existing:   []runtime.Object{cmDifferentData},
				wantEvents: []string{configmapUpdatedEvent},
			},
			in:     cm,
			want:   cm,
			eqFunc: DefaultConfigMapEqual,
		},
		{
			commonCase: commonCase{
				name:      "configmap update error",
				reactions: []clientgotesting.ReactionFunc{configmapUpdateFailure},
				existing:  []runtime.Object{cmDifferentData},
				wantErr:   true,
			},
			in:     cm,
			eqFunc: DefaultConfigMapEqual,
		},
		{
			commonCase: commonCase{
				name:       "configmap custom eqFunc, force update",
				existing:   []runtime.Object{cm},
				wantEvents: []string{configmapUpdatedEvent},
			},
			in:   cm,
			want: cm,
			eqFunc: func(cm1, cm2 *corev1.ConfigMap) bool {
				return false
			},
		},
		{
			commonCase: commonCase{
				name:     "configmap custom eqFunc, nothing to do",
				existing: []runtime.Object{cmDifferentData},
			},
			in:   cm,
			want: cmDifferentData,
			eqFunc: func(cm1, cm2 *corev1.ConfigMap) bool {
				return true
			},
		},
		{
			commonCase: commonCase{
				name:     "eqFunc missing error",
				existing: []runtime.Object{cm},
				wantErr:  true,
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
			out, err := rec.ReconcileConfigMap(context.Background(), obj, test.in, test.eqFunc)

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
