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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

func TestBrokerCell_Validate(t *testing.T) {
	tests := []struct {
		name       string
		brokerCell BrokerCell
		want       *apis.FieldError
	}{
		{
			name: "Valid spec",
			brokerCell: BrokerCell{
				Spec: MakeDefaultBrokerCellSpec(),
			},
			want: nil,
		}, {
			name: "Memory request should not exceed the memory limit",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidMemoryRequest := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidMemoryRequest.Components.Ingress
					testComponent.Resources.Limits.Memory = "2000Mi"
					testComponent.Resources.Requests.Memory = "2001Mi"
					return brokerCellWithInvalidMemoryRequest
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				fe := apis.ErrInvalidValue("2001Mi", "spec.components.ingress.resources.requests.memory")
				fe.Details = "Resource request should not exceed the resource limit"
				fieldErrors = fieldErrors.Also(fe)
				return fieldErrors

			}(),
		}, {
			name: "AvgMemoryUsage should not exceed the memory limit",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidMemoryRequest := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidMemoryRequest.Components.Ingress
					testComponent.Resources.Limits.Memory = "1000Mi"
					testComponent.AvgMemoryUsage = ptr.String("1001Mi")
					return brokerCellWithInvalidMemoryRequest
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				fe := apis.ErrInvalidValue("1001Mi", "spec.components.ingress.AvgMemoryUsage")
				fe.Details = "AvgMemoryUsage should not exceed the memory limit"
				fieldErrors = fieldErrors.Also(fe)
				return fieldErrors
			}(),
		}, {
			name: "CPU request should not exceed the cpu limit",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidCPURequest := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidCPURequest.Components.Ingress
					testComponent.Resources.Limits.CPU = "1000m"
					testComponent.Resources.Requests.CPU = "1001m"
					return brokerCellWithInvalidCPURequest
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				fe := apis.ErrInvalidValue("1001m", "spec.components.ingress.resources.requests.cpu")
				fe.Details = "Resource request should not exceed the resource limit"
				fieldErrors = fieldErrors.Also(fe)
				return fieldErrors

			}(),
		}, {
			name: "At least one of the autoscaling metrics should be specified",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidCPURequest := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidCPURequest.Components.Ingress
					testComponent.AvgCPUUtilization = nil
					testComponent.AvgMemoryUsage = nil
					return brokerCellWithInvalidCPURequest
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				fe := apis.ErrInvalidValue(nil, "spec.components.ingress")
				fe.Details = "At least one of the autoscaling metrics (AvgCPUUtilization, AvgMemoryUsage) should be specified"
				fieldErrors = fieldErrors.Also(fe)
				return fieldErrors

			}(),
		}, {
			name: "Invalid quantities are catched",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidMemoryLimit := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidMemoryLimit.Components.Ingress
					testComponent.Resources.Requests.CPU = "invalid_requests_cpu"
					testComponent.Resources.Limits.CPU = "invalid_limits_cpu"
					testComponent.Resources.Requests.Memory = "invalid_requests_memory"
					testComponent.Resources.Limits.Memory = "invalid_limits_memory"
					testComponent.AvgMemoryUsage = ptr.String("invalid_value_AvgMemoryUsage")
					return brokerCellWithInvalidMemoryLimit
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				unexpectedQuantityFormatErrorDetail := "The quantity is specified in an unexpected format"
				cpuRequestFE := apis.ErrInvalidValue("invalid_requests_cpu", "spec.components.ingress.resources.requests.cpu")
				cpuRequestFE.Details = unexpectedQuantityFormatErrorDetail
				fieldErrors = fieldErrors.Also(cpuRequestFE)
				cpuLimitFE := apis.ErrInvalidValue("invalid_limits_cpu", "spec.components.ingress.resources.limits.cpu")
				cpuLimitFE.Details = unexpectedQuantityFormatErrorDetail
				fieldErrors = fieldErrors.Also(cpuLimitFE)
				memoryRequestFE := apis.ErrInvalidValue("invalid_requests_memory", "spec.components.ingress.resources.requests.memory")
				memoryRequestFE.Details = unexpectedQuantityFormatErrorDetail
				fieldErrors = fieldErrors.Also(memoryRequestFE)
				memoryLimitsFE := apis.ErrInvalidValue("invalid_limits_memory", "spec.components.ingress.resources.limits.memory")
				memoryLimitsFE.Details = unexpectedQuantityFormatErrorDetail
				fieldErrors = fieldErrors.Also(memoryLimitsFE)
				avgMemoryUsageFE := apis.ErrInvalidValue("invalid_value_AvgMemoryUsage", "spec.components.ingress.AvgMemoryUsage")
				avgMemoryUsageFE.Details = unexpectedQuantityFormatErrorDetail
				fieldErrors = fieldErrors.Also(avgMemoryUsageFE)
				return fieldErrors
			}(),
		},
		{
			name: "MinRelicas can not be larger than MaxReplicas",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidMinReplicas := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidMinReplicas.Components.Ingress
					testComponent.MinReplicas = ptr.Int32(11)
					testComponent.MaxReplicas = ptr.Int32(10)
					return brokerCellWithInvalidMinReplicas
				}()),
			},
			want: func() *apis.FieldError {
				var fieldErrors *apis.FieldError
				fe := apis.ErrInvalidValue(11, "spec.components.ingress.MinReplicas")
				fe.Details = "MinReplicas value can not exceed the value of MaxReplicas"
				fieldErrors = fieldErrors.Also(fe)
				return fieldErrors

			}(),
		},
		{
			name: "Empty quantities are supported",
			brokerCell: BrokerCell{
				Spec: (func() BrokerCellSpec {
					brokerCellWithInvalidMemoryLimit := MakeDefaultBrokerCellSpec()
					testComponent := brokerCellWithInvalidMemoryLimit.Components.Ingress
					testComponent.Resources.Limits.CPU = ""
					return brokerCellWithInvalidMemoryLimit
				}()),
			},
			want: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.brokerCell.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("failed to get expected (-want, +got) = %v", diff)
			}
		})
	}
}
