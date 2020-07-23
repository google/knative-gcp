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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	gcpauthtesthelper "github.com/google/knative-gcp/pkg/apis/configs/gcpauth/testhelper"
	"github.com/google/knative-gcp/pkg/apis/duck"
	testingMetadataClient "github.com/google/knative-gcp/pkg/gclient/metadata/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestBrokerCell_SetDefaults(t *testing.T) {
	defaultMinReplicas, defaultMaxRepicas := ptr.Int32(1), ptr.Int32(10)
	ingressMinReplicas, ingressMaxReplicas := ptr.Int32(2), ptr.Int32(3)
	ingressAvgCPUUtilization, ingressAvgMemoryUsage := ptr.Int32(100), ptr.String("1000Mi")
	fanoutMinReplicas, fanoutMaxReplicas := ptr.Int32(4), ptr.Int32(5)
	fanoutAvgCPUUtilization, fanoutAvgMemoryUsage := ptr.Int32(101), ptr.String("10000Mi")
	retryMinReplicas, retryMaxReplicas := ptr.Int32(6), ptr.Int32(7)
	retryAvgCPUUtilization, retryAvgMemoryUsage := ptr.Int32(102), ptr.String("100000Mi")
	tests := []struct {
		name  string
		start *BrokerCell
		want  *BrokerCell
	}{{
		name: "Spec defaults",
		start: &BrokerCell{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: BrokerCellSpec{},
		},
		want: &BrokerCell{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
						AvgMemoryUsage: ptr.String(avgMemoryUsage),
						MaxReplicas: defaultMaxRepicas,
						MinReplicas: defaultMinReplicas,
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
						AvgMemoryUsage: ptr.String(avgMemoryUsageIngress),
						MaxReplicas: defaultMaxRepicas,
						MinReplicas: defaultMinReplicas,
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
						AvgMemoryUsage: ptr.String(avgMemoryUsage),
						MaxReplicas: defaultMaxRepicas,
						MinReplicas: defaultMinReplicas,
					},
				},
			},
		},
	}, {
		name: "Spec set",
		start: &BrokerCell{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: ComponentParameters{
						AvgCPUUtilization: fanoutAvgCPUUtilization,
						AvgMemoryUsage: fanoutAvgMemoryUsage,
						MaxReplicas: fanoutMaxReplicas,
						MinReplicas: fanoutMinReplicas,
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ingressAvgCPUUtilization,
						AvgMemoryUsage: ingressAvgMemoryUsage,
						MaxReplicas: ingressMaxReplicas,
						MinReplicas: ingressMinReplicas,
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: retryAvgCPUUtilization,
						AvgMemoryUsage: retryAvgMemoryUsage,
						MaxReplicas: retryMaxReplicas,
						MinReplicas: retryMinReplicas,
					},
				},
			},
		},
		want: &BrokerCell{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					duck.ClusterNameAnnotation: testingMetadataClient.FakeClusterName,
				},
			},
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: ComponentParameters{
						AvgCPUUtilization: fanoutAvgCPUUtilization,
						AvgMemoryUsage: fanoutAvgMemoryUsage,
						MaxReplicas: fanoutMaxReplicas,
						MinReplicas: fanoutMinReplicas,
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ingressAvgCPUUtilization,
						AvgMemoryUsage: ingressAvgMemoryUsage,
						MaxReplicas: ingressMaxReplicas,
						MinReplicas: ingressMinReplicas,
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: retryAvgCPUUtilization,
						AvgMemoryUsage: retryAvgMemoryUsage,
						MaxReplicas: retryMaxReplicas,
						MinReplicas: retryMinReplicas,
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.start
			got.SetDefaults(gcpauthtesthelper.ContextWithDefaults())

			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}
