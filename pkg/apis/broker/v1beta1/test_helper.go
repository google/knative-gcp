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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

func (t testHelper) ReadyBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.SetAddress(apis.HTTP("example.com"))
	bs.MarkSubscriptionReady()
	bs.MarkTopicReady()
	bs.MarkBrokerCellReady()
	return bs
}

func (t testHelper) UnconfiguredBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	return bs
}

func (t testHelper) UnknownBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.InitializeConditions()
	return bs
}

func (t testHelper) FalseBrokerStatus() *BrokerStatus {
	bs := &BrokerStatus{}
	bs.SetAddress(nil)
	return bs
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

func (t testHelper) ReadyDependencyStatus() *duckv1.Source {
	src := &duckv1.Source{}
	src.Status.SetConditions(apis.Conditions{{
		Type:   "Ready",
		Status: corev1.ConditionTrue,
	}})
	return src
}

func (t testHelper) UnconfiguredDependencyStatus() *duckv1.Source {
	src := &duckv1.Source{}
	return src
}

func (t testHelper) UnknownDependencyStatus() *duckv1.Source {
	src := &duckv1.Source{}
	src.Status.SetConditions(apis.Conditions{{
		Type:   "Ready",
		Status: corev1.ConditionUnknown,
	}})
	return src
}

func (t testHelper) FalseDependencyStatus() *duckv1.Source {
	src := &duckv1.Source{}
	src.Status.SetConditions(apis.Conditions{{
		Type:   "Ready",
		Status: corev1.ConditionFalse,
	}})
	return src
}
