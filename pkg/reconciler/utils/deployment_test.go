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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/ptr"
	pkgreconcilertesting "knative.dev/pkg/reconciler/testing"
)

const (
	deploymentCreatedEvent = "Normal DeploymentCreated Created deployment testns/test"
	deploymentUpdatedEvent = "Normal DeploymentUpdated Updated deployment testns/test"
)

var (
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       appsv1.DeploymentSpec{MinReadySeconds: 10},
	}
	deploymentDifferentSpec = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       appsv1.DeploymentSpec{MinReadySeconds: 20},
	}
	deploymentWithReplicas = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "testns", Name: "test"},
		Spec:       appsv1.DeploymentSpec{MinReadySeconds: 10, Replicas: ptr.Int32(3)},
	}

	deploymentCreateFailure = pkgreconcilertesting.InduceFailure("create", "deployments")
	deploymentUpdateFailure = pkgreconcilertesting.InduceFailure("update", "deployments")
)

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
		{
			commonCase: commonCase{
				name:     "deployment exists with different replicas",
				existing: []runtime.Object{deploymentWithReplicas},
			},
			in:   deployment,
			want: deploymentWithReplicas,
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
			out, err := rec.ReconcileDeployment(context.Background(), obj, test.in)

			tr.verify(t, test.commonCase, err)

			if diff := cmp.Diff(out, test.want); diff != "" {
				t.Errorf("Unexpected reconciler result (-got, +want): %s", diff)
				return
			}
		})
	}
}
