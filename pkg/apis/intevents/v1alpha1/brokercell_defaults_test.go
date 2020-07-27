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
	customCPURequest, customCPULimit := "112m", "113m"
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
			Spec: makeDefaultBrokerCellSpec(),
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
						CPURequest:        ptr.String(fanoutSpecPrefix + customCPURequest),
						CPULimit:          ptr.String(fanoutSpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(fanoutSpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(fanoutSpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: ComponentParameters{
						CPURequest:        ptr.String(ingressSpecPrefix + customCPURequest),
						CPULimit:          ptr.String(ingressSpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(ingressSpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(ingressSpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: ComponentParameters{
						CPURequest:        ptr.String(retrySpecPrefix + customCPURequest),
						CPULimit:          ptr.String(retrySpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(retrySpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(retrySpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(retrySpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(retrySpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(retrySpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(retrySpecShift * customMinReplicas)),
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
						CPURequest:        ptr.String(fanoutSpecPrefix + customCPURequest),
						CPULimit:          ptr.String(fanoutSpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(fanoutSpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(fanoutSpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: ComponentParameters{
						CPURequest:        ptr.String(ingressSpecPrefix + customCPURequest),
						CPULimit:          ptr.String(ingressSpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(ingressSpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(ingressSpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: ComponentParameters{
						CPURequest:        ptr.String(retrySpecPrefix + customCPURequest),
						CPULimit:          ptr.String(retrySpecPrefix + customCPULimit),
						MemoryRequest:     ptr.String(retrySpecPrefix + customMemoryRequest),
						MemoryLimit:       ptr.String(retrySpecPrefix + customMemoryLimit),
						AvgCPUUtilization: ptr.Int32(int32(retrySpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(retrySpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(retrySpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(retrySpecShift * customMinReplicas)),
					},
				},
			},
		},
	}, {
		name: "Target average memory consumption can not exceed the memory limit",
		start: &BrokerCell{
			Spec: (func() BrokerCellSpec {
				brokerCellWithInvalidTargetAvgMemory := makeDefaultBrokerCellSpec()
				testComponent := &brokerCellWithInvalidTargetAvgMemory.Components.Fanout
				testComponent.MemoryRequest = ptr.String("500Mi")
				testComponent.MemoryLimit = ptr.String("2000Mi")
				testComponent.AvgMemoryUsage = ptr.String("2001Mi") // Falls beyond the limit
				return brokerCellWithInvalidTargetAvgMemory
			}()),
		},
		want: &BrokerCell{
			Spec: (func() BrokerCellSpec {
				brokerCellWithCorrectedTargetAvgMemory := makeDefaultBrokerCellSpec()
				testComponent := &brokerCellWithCorrectedTargetAvgMemory.Components.Fanout
				testComponent.MemoryRequest = ptr.String("500Mi")
				testComponent.MemoryLimit = ptr.String("2000Mi")
				testComponent.AvgMemoryUsage = ptr.String("1000Mi") // Auto-selected
				return brokerCellWithCorrectedTargetAvgMemory
			}()),
		},
	}, {
		name: "Unspecified target memory consumption is auto-selected with respect to the memory limit",
		start: &BrokerCell{
			Spec: (func() BrokerCellSpec {
				brokerCellWithInvalidTargetAvgMemory := makeDefaultBrokerCellSpec()
				testComponent := &brokerCellWithInvalidTargetAvgMemory.Components.Fanout
				testComponent.MemoryLimit = ptr.String("10000Mi")
				testComponent.AvgMemoryUsage = nil
				return brokerCellWithInvalidTargetAvgMemory
			}()),
		},
		want: &BrokerCell{
			Spec: (func() BrokerCellSpec {
				brokerCellWithCorrectedTargetAvgMemory := makeDefaultBrokerCellSpec()
				testComponent := &brokerCellWithCorrectedTargetAvgMemory.Components.Fanout
				testComponent.MemoryLimit = ptr.String("10000Mi")
				testComponent.AvgMemoryUsage = ptr.String("5000Mi") // Auto-selected
				return brokerCellWithCorrectedTargetAvgMemory
			}()),
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

func makeDefaultBrokerCellSpec() BrokerCellSpec {
	memoryLimit, memoryLimitIngress := "3000Mi", "1000Mi"

	defaultBrokerCell := BrokerCellSpec{
		ComponentsParametersSpec{
			Fanout: ComponentParameters{
				CPURequest:        ptr.String(cpuRequestFanout),
				CPULimit:          ptr.String(cpuLimit),
				MemoryRequest:     ptr.String(memoryRequest),
				MemoryLimit:       ptr.String(memoryLimit),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
				AvgMemoryUsage:    ptr.String(avgMemoryUsage),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
			Ingress: ComponentParameters{
				CPURequest:        ptr.String(cpuRequest),
				CPULimit:          ptr.String(cpuLimit),
				MemoryRequest:     ptr.String(memoryRequest),
				MemoryLimit:       ptr.String(memoryLimitIngress),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
				AvgMemoryUsage:    ptr.String(avgMemoryUsageIngress),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
			Retry: ComponentParameters{
				CPURequest:        ptr.String(cpuRequest),
				CPULimit:          ptr.String(cpuLimit),
				MemoryRequest:     ptr.String(memoryRequest),
				MemoryLimit:       ptr.String(memoryLimit),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
				AvgMemoryUsage:    ptr.String(avgMemoryUsage),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
		},
	}
	return defaultBrokerCell
}
