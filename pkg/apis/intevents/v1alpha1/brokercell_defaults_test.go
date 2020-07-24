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
	ingressSpecPrefix, fanoutSpecPrefix, retrySpecPrefix := "10", "100", "1000"
	ingressSpecShift, fanoutSpecShift, retrySpecShift := 10, 100, 1000
	customMinReplicas, customMaxReplicas := 4, 5
	customAvgCPUUtilization, customAvgMemoryUsage := 101, "111Mi"
	customCPURequest, customCPULimit := "112Mi", "113Mi"
	customMemoryRequest, customMemoryLimit := "114Mi", "115Mi"

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
						CPURequest: ptr.String(cpuRequestFanout),
						CPULimit: ptr.String(cpuLimit),
						MemoryRequest: ptr.String(memoryRequest),
						MemoryLimit: ptr.String(memoryLimit),
						MaxReplicas: ptr.Int32(maxReplicas),
						MinReplicas: ptr.Int32(minReplicas),
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
						AvgMemoryUsage: ptr.String(avgMemoryUsageIngress),
						CPURequest: ptr.String(cpuRequest),
						CPULimit: ptr.String(cpuLimit),
						MemoryRequest: ptr.String(memoryRequest),
						MemoryLimit: ptr.String(memoryLimitIngress),
						MaxReplicas: ptr.Int32(maxReplicas),
						MinReplicas: ptr.Int32(minReplicas),
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
						AvgMemoryUsage: ptr.String(avgMemoryUsage),
						CPURequest: ptr.String(cpuRequest),
						CPULimit: ptr.String(cpuLimit),
						MemoryRequest: ptr.String(memoryRequest),
						MemoryLimit: ptr.String(memoryLimit),
						MaxReplicas: ptr.Int32(maxReplicas),
						MinReplicas: ptr.Int32(minReplicas),
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
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(fanoutSpecPrefix + customCPURequest),
						CPULimit: ptr.String(fanoutSpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(fanoutSpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(fanoutSpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(ingressSpecPrefix + customCPURequest),
						CPULimit: ptr.String(ingressSpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(ingressSpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(ingressSpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(int32(retrySpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(retrySpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(retrySpecPrefix + customCPURequest),
						CPULimit: ptr.String(retrySpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(retrySpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(retrySpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(retrySpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(retrySpecShift * customMinReplicas)),
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
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(fanoutSpecPrefix + customCPURequest),
						CPULimit: ptr.String(fanoutSpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(fanoutSpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(fanoutSpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(ingressSpecPrefix + customCPURequest),
						CPULimit: ptr.String(ingressSpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(ingressSpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(ingressSpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: ComponentParameters{
						AvgCPUUtilization: ptr.Int32(int32(retrySpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage: ptr.String(retrySpecPrefix + customAvgMemoryUsage),
						CPURequest: ptr.String(retrySpecPrefix + customCPURequest),
						CPULimit: ptr.String(retrySpecPrefix + customCPULimit),
						MemoryRequest: ptr.String(retrySpecPrefix + customMemoryRequest),
						MemoryLimit: ptr.String(retrySpecPrefix + customMemoryLimit),
						MaxReplicas: ptr.Int32(int32(retrySpecShift * customMaxReplicas)),
						MinReplicas: ptr.Int32(int32(retrySpecShift * customMinReplicas)),
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

