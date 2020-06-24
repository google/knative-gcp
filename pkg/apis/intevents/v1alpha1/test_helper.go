/*
Copyright 2020 Google LLC.

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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

func (t testHelper) UnavailableEndpoints() *corev1.Endpoints {
	ep := &corev1.Endpoints{}
	ep.Name = "unavailable"
	ep.Subsets = []corev1.EndpointSubset{{
		NotReadyAddresses: []corev1.EndpointAddress{{
			IP: "127.0.0.1",
		}},
	}}
	return ep
}
func (t testHelper) AvailableEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "available",
		},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{
				IP: "127.0.0.1",
			}},
		}},
	}
}

func (t testHelper) AvailableDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "available"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
}

func (t testHelper) UnavailableDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "unavailable"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: "False",
		},
	}
	return d
}

func (t testHelper) UnknownDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{}
	d.Name = "unknown"
	d.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: "Unknown",
		},
	}
	return d
}

func (t testHelper) ReadyBrokerCellStatus() *BrokerCellStatus {
	bs := &BrokerCellStatus{}
	bs.PropagateIngressAvailability(t.AvailableEndpoints())
	bs.SetIngressTemplate("http://localhost")
	bs.PropagateFanoutAvailability(t.AvailableDeployment())
	bs.PropagateRetryAvailability(t.AvailableDeployment())
	bs.MarkTargetsConfigReady()
	return bs
}
