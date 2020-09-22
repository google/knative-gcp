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

	"knative.dev/pkg/ptr"
)

const (
	avgCPUUtilizationFanout  int32 = 30
	avgCPUUtilizationIngress int32 = 95
	avgCPUUtilizationRetry   int32 = 95
	// The limit we set (for Fanout and Retry) is 3000Mi which is mostly used
	// to prevent surging memory usage causing OOM.
	// Here we only set half of the limit so that in case of surging memory
	// usage, HPA could have enough time to kick in.
	// See: https://github.com/google/knative-gcp/issues/1265
	avgMemoryUsageFanout  string = "1000Mi"
	avgMemoryUsageIngress string = "1500Mi"
	avgMemoryUsageRetry   string = "1500Mi"
	cpuRequestFanout      string = "1500m"
	cpuRequestIngress     string = "2000m"
	cpuRequestRetry       string = "1000m"
	cpuLimitFanout        string = ""
	cpuLimitIngress       string = ""
	cpuLimitRetry         string = ""
	memoryRequestFanout   string = "3000Mi"
	memoryRequestIngress  string = "2000Mi"
	memoryRequestRetry    string = "500Mi"
	memoryLimitFanout     string = "3000Mi"
	memoryLimitIngress    string = "2000Mi"
	memoryLimitRetry      string = "3000Mi"
	minReplicas           int32  = 1
	maxReplicas           int32  = 10
)

// SetDefaults sets the default field values for a BrokerCell.
func (bc *BrokerCell) SetDefaults(ctx context.Context) {
	// Set defaults for the Spec.Components values.
	bc.Spec.SetDefaults(ctx)
}

// SetDefaults sets the default field values for a BrokerCellSpec.
func (bcs *BrokerCellSpec) SetDefaults(ctx context.Context) {
	// Fanout defaults
	if bcs.Components.Fanout == nil {
		bcs.Components.Fanout = makeComponent(cpuRequestFanout, cpuLimitFanout, memoryRequestFanout, memoryLimitFanout, avgCPUUtilizationFanout, avgMemoryUsageFanout)
	}
	bcs.Components.Fanout.setAutoScalingDefaults()
	// Ingress defaults
	if bcs.Components.Ingress == nil {
		bcs.Components.Ingress = makeComponent(cpuRequestIngress, cpuLimitIngress, memoryRequestIngress, memoryLimitIngress, avgCPUUtilizationIngress, avgMemoryUsageIngress)
	}
	bcs.Components.Ingress.setAutoScalingDefaults()
	// Retry defaults
	if bcs.Components.Retry == nil {
		bcs.Components.Retry = makeComponent(cpuRequestRetry, cpuLimitRetry, memoryRequestRetry, memoryLimitRetry, avgCPUUtilizationRetry, avgMemoryUsageRetry)
	}
	bcs.Components.Retry.setAutoScalingDefaults()
}

func makeComponent(cpuRequest, cpuLimit, memoryRequest, memoryLimit string, avgCPUUtilization int32, targetMemoryUsage string) *ComponentParameters {
	return &ComponentParameters{
		CPURequest:        cpuRequest,
		CPULimit:          cpuLimit,
		MemoryRequest:     memoryRequest,
		MemoryLimit:       memoryLimit,
		AvgCPUUtilization: ptr.Int32(avgCPUUtilization),
		AvgMemoryUsage:    ptr.String(targetMemoryUsage),
	}
}

func (componentParams *ComponentParameters) setAutoScalingDefaults() {
	if componentParams.MinReplicas == nil {
		componentParams.MinReplicas = ptr.Int32(minReplicas)
	}
	if componentParams.MaxReplicas == nil {
		componentParams.MaxReplicas = ptr.Int32(maxReplicas)
	}
}
