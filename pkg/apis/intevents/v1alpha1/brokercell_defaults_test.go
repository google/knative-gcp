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
			Spec: MakeDefaultBrokerCellSpec(),
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
					Fanout: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, fanoutSpecPrefix),
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, ingressSpecPrefix),
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, retrySpecPrefix),
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
					Fanout: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, fanoutSpecPrefix),
						AvgCPUUtilization: ptr.Int32(int32(fanoutSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(fanoutSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(fanoutSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(fanoutSpecShift * customMinReplicas)),
					},
					Ingress: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, ingressSpecPrefix),
						AvgCPUUtilization: ptr.Int32(int32(ingressSpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(ingressSpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(ingressSpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(ingressSpecShift * customMinReplicas)),
					},
					Retry: &ComponentParameters{
						Resources: makeResourcesSpec(customCPURequest, customCPULimit, customMemoryRequest,
							customMemoryLimit, retrySpecPrefix),
						AvgCPUUtilization: ptr.Int32(int32(retrySpecShift * customAvgCPUUtilization)),
						AvgMemoryUsage:    ptr.String(retrySpecPrefix + customAvgMemoryUsage),
						MaxReplicas:       ptr.Int32(int32(retrySpecShift * customMaxReplicas)),
						MinReplicas:       ptr.Int32(int32(retrySpecShift * customMinReplicas)),
					},
				},
			},
		},
	}, {
		name: "Defaulting for resource specification is not applied when some of the parameters are specified",
		start: &BrokerCell{
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: &ComponentParameters{
						Resources: ResourceSpecification{
							Requests: SystemResource{
								CPU: "10000",
							},
						},
					},
				},
			},
		},
		want: &BrokerCell{
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: (&ComponentParameters{
						Resources: ResourceSpecification{
							Requests: SystemResource{
								CPU:    "10000",
								Memory: "",
							},
							Limits: SystemResource{
								CPU:    "",
								Memory: "",
							},
						},
						AvgCPUUtilization: nil,
						AvgMemoryUsage:    nil,
					}).withDefaultReplicas(),
					Ingress: makeComponent(cpuRequestIngress, cpuLimitIngress, memoryRequestIngress, memoryLimitIngress, avgCPUUtilizationIngress, avgMemoryUsageIngress).withDefaultReplicas(),
					Retry:   makeComponent(cpuRequestRetry, cpuLimitRetry, memoryRequestRetry, memoryLimitRetry, avgCPUUtilizationRetry, avgMemoryUsageRetry).withDefaultReplicas(),
				},
			},
		},
	}, {
		name: "Defaulting for resource specification is not applied when a target CPU or memory parameter is specified",
		start: &BrokerCell{
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: &ComponentParameters{
						AvgCPUUtilization: ptr.Int32(95),
					},
				},
			},
		},
		want: &BrokerCell{
			Spec: BrokerCellSpec{
				ComponentsParametersSpec{
					Fanout: (&ComponentParameters{
						AvgCPUUtilization: ptr.Int32(95),
						AvgMemoryUsage:    nil,
						Resources:         ResourceSpecification{},
					}).withDefaultReplicas(),
					Ingress: makeComponent(cpuRequestIngress, cpuLimitIngress, memoryRequestIngress, memoryLimitIngress, avgCPUUtilizationIngress, avgMemoryUsageIngress).withDefaultReplicas(),
					Retry:   makeComponent(cpuRequestRetry, cpuLimitRetry, memoryRequestRetry, memoryLimitRetry, avgCPUUtilizationRetry, avgMemoryUsageRetry).withDefaultReplicas(),
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

func MakeDefaultBrokerCellSpec() BrokerCellSpec {
	memoryLimitFanout, memoryLimitIngress, memoryLimitRetry := "3000Mi", "1000Mi", "3000Mi"

	defaultBrokerCell := BrokerCellSpec{
		ComponentsParametersSpec{
			Fanout: &ComponentParameters{
				Resources:         makeResourcesSpec(cpuRequestFanout, cpuLimitFanout, memoryRequestFanout, memoryLimitFanout, ""),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilizationFanout),
				AvgMemoryUsage:    ptr.String(avgMemoryUsageFanout),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
			Ingress: &ComponentParameters{
				Resources:         makeResourcesSpec(cpuRequestIngress, cpuLimitIngress, memoryRequestIngress, memoryLimitIngress, ""),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilizationIngress),
				AvgMemoryUsage:    ptr.String(avgMemoryUsageIngress),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
			Retry: &ComponentParameters{
				Resources:         makeResourcesSpec(cpuRequestRetry, cpuLimitRetry, memoryRequestRetry, memoryLimitRetry, ""),
				AvgCPUUtilization: ptr.Int32(avgCPUUtilizationRetry),
				AvgMemoryUsage:    ptr.String(avgMemoryUsageRetry),
				MaxReplicas:       ptr.Int32(maxReplicas),
				MinReplicas:       ptr.Int32(minReplicas),
			},
		},
	}
	return defaultBrokerCell
}

func makeResourcesSpec(cpuRequest, cpuLimit, memoryRequest, memoryLimit, prefix string) ResourceSpecification {
	resourceSpec := ResourceSpecification{
		Requests: SystemResource{
			CPU:    prefix + cpuRequest,
			Memory: prefix + memoryRequest,
		},
		Limits: SystemResource{
			CPU:    prefix + cpuLimit,
			Memory: prefix + memoryLimit,
		},
	}

	return resourceSpec
}

func (componentParams *ComponentParameters) withDefaultReplicas() *ComponentParameters {
	componentParams.MinReplicas = ptr.Int32(minReplicas)
	componentParams.MaxReplicas = ptr.Int32(maxReplicas)
	return componentParams
}
