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

	quantityutil "github.com/google/knative-gcp/pkg/utils/resource"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/ptr"
)

const (
	avgCPUUtilization int32 = 95
	// The limit we set (for Fanout and Retry) is 3000Mi which is mostly used
	// to prevent surging memory usage causing OOM.
	// Here we only set half of the limit so that in case of surging memory
	// usage, HPA could have enough time to kick in.
	// See: https://github.com/google/knative-gcp/issues/1265
	avgMemoryUsage                         string  = "1500Mi"
	avgMemoryUsageIngress                  string  = "700Mi"
	cpuRequest                             string  = "1000m"
	cpuRequestFanout                       string  = "1500m"
	cpuLimit                               string  = ""
	memoryRequest                          string  = "500Mi"
	memoryLimitToRequestCoefficient        float64 = 6.0
	memoryLimitToRequestCoefficientIngress float64 = 2.0
	targetMemoryUsageCoefficient           float64 = 0.5
	targetMemoryUsageCoefficientIngress    float64 = 0.7
	minReplicas                            int32   = 1
	maxReplicas                            int32   = 10
)

// SetDefaults sets the default field values for a BrokerCell.
func (bc *BrokerCell) SetDefaults(ctx context.Context) {
	// Set defaults for the Spec.Components values.
	bc.Spec.SetDefaults(ctx)
}

// SetDefaults sets the default field values for a BrokerCellSpec.
func (bcs *BrokerCellSpec) SetDefaults(ctx context.Context) {
	// Fanout defaults
	bcs.Components.Fanout.SetCPUDefaults(cpuRequestFanout, cpuLimit)
	bcs.Components.Fanout.SetMemoryDefaults(memoryLimitToRequestCoefficient)
	bcs.Components.Fanout.SetAutoScalingDefaults(targetMemoryUsageCoefficient)
	// Retry defaults
	bcs.Components.Retry.SetCPUDefaults(cpuRequest, cpuLimit)
	bcs.Components.Retry.SetMemoryDefaults(memoryLimitToRequestCoefficient)
	bcs.Components.Retry.SetAutoScalingDefaults(targetMemoryUsageCoefficient)
	// Ingress defaults
	bcs.Components.Ingress.SetCPUDefaults(cpuRequest, cpuLimit)
	bcs.Components.Ingress.SetMemoryDefaults(memoryLimitToRequestCoefficientIngress)
	bcs.Components.Ingress.SetAutoScalingDefaults(targetMemoryUsageCoefficientIngress)
}

// SetMemoryDefaults sets the memory consumption related default field values for ComponentParameters.
func (componentParams *ComponentParameters) SetMemoryDefaults(memoryLimitToRequestCoefficient float64) {
	if componentParams.MemoryRequest == nil {
		componentParams.MemoryRequest = ptr.String(memoryRequest)
	}
	requestedMemoryQuantity := resource.MustParse(*componentParams.MemoryRequest)
	if componentParams.MemoryLimit == nil {
		autoSelectedLimit := quantityutil.MultiplyQuantity(requestedMemoryQuantity, memoryLimitToRequestCoefficient)
		componentParams.MemoryLimit = ptr.String(autoSelectedLimit.String())
	}
	// Make sure the limit is not lower than what's requested
	memoryLimitQuantity := resource.MustParse(*componentParams.MemoryLimit)
	if memoryLimitQuantity.Cmp(requestedMemoryQuantity) < 0 {
		componentParams.MemoryLimit = ptr.String(requestedMemoryQuantity.String())
		memoryLimitQuantity = requestedMemoryQuantity
	}
}

// SetCPUDefaults sets the CPU consumption related default field values for ComponentParameters.
func (componentParams *ComponentParameters) SetCPUDefaults(defaultCPURequest, defaultCPULimit string) {
	if componentParams.CPURequest == nil {
		componentParams.CPURequest = ptr.String(defaultCPURequest)
	}
	if componentParams.CPULimit == nil {
		componentParams.CPULimit = ptr.String(defaultCPULimit)
	}
}

// SetAutoScalingDefaults sets the autoscaling-related default field values for ComponentParameters.
func (componentParams *ComponentParameters) SetAutoScalingDefaults(targetMemoryUsageCoefficient float64) {
	if componentParams.MemoryLimit == nil {
		panic("Memory limit should be specified on the component before setting auto-scaling parameters")
	}
	if componentParams.MinReplicas == nil {
		componentParams.MinReplicas = ptr.Int32(minReplicas)
	}
	if componentParams.MaxReplicas == nil {
		componentParams.MaxReplicas = ptr.Int32(maxReplicas)
	}
	if componentParams.AvgCPUUtilization == nil {
		componentParams.AvgCPUUtilization = ptr.Int32(avgCPUUtilization)
	}
	// If target average consumption for the auto-scaler is not explicitly specified or falls beyond reasonable range,
	// default it based on the memory limit
	memoryLimitQuantity := resource.MustParse(*componentParams.MemoryLimit)
	if componentParams.AvgMemoryUsage == nil || memoryLimitQuantity.Cmp(resource.MustParse(*componentParams.AvgMemoryUsage)) < 0 {
		autoSelectedAvgMemoryUsage := quantityutil.MultiplyQuantity(memoryLimitQuantity, targetMemoryUsageCoefficient)
		componentParams.AvgMemoryUsage = ptr.String(autoSelectedAvgMemoryUsage.String())
	}
}
