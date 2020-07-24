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
	avgCPUUtilization int32 = 95
	// The limit we set (for Fanout and Retry) is 3000Mi which is mostly used
	// to prevent surging memory usage causing OOM.
	// Here we only set half of the limit so that in case of surging memory
	// usage, HPA could have enough time to kick in.
	// See: https://github.com/google/knative-gcp/issues/1265
	avgMemoryUsage        string = "1500Mi"
	avgMemoryUsageIngress string = "700Mi"
	cpuRequest            string = "1000Mi"
	cpuRequestFanout      string = "1500Mi"
	cpuLimit              string = ""
	memoryRequest         string = "500Mi"
	memoryLimit           string = "3000Mi"
	memoryLimitIngress    string = "1000Mi"
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
	if bcs.Components.Fanout.AvgCPUUtilization == nil {
		bcs.Components.Fanout.AvgCPUUtilization = ptr.Int32(avgCPUUtilization)
	}
	if bcs.Components.Fanout.AvgMemoryUsage == nil {
		bcs.Components.Fanout.AvgMemoryUsage = ptr.String(avgMemoryUsage)
	}
	if bcs.Components.Fanout.CPURequest == nil {
		bcs.Components.Fanout.CPURequest = ptr.String(cpuRequestFanout)
	}
	if bcs.Components.Fanout.CPULimit == nil {
		bcs.Components.Fanout.CPULimit = ptr.String(cpuLimit)
	}
	if bcs.Components.Fanout.MemoryRequest == nil {
		bcs.Components.Fanout.MemoryRequest = ptr.String(memoryRequest)
	}
	if bcs.Components.Fanout.MemoryLimit == nil {
		bcs.Components.Fanout.MemoryLimit = ptr.String(memoryLimit)
	}
	if bcs.Components.Fanout.MinReplicas == nil {
		bcs.Components.Fanout.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Fanout.MaxReplicas == nil {
		bcs.Components.Fanout.MaxReplicas = ptr.Int32(maxReplicas)
	}

	// Retry defaults
	if bcs.Components.Retry.AvgCPUUtilization == nil {
		bcs.Components.Retry.AvgCPUUtilization = ptr.Int32(avgCPUUtilization)
	}
	if bcs.Components.Retry.AvgMemoryUsage == nil {
		bcs.Components.Retry.AvgMemoryUsage = ptr.String(avgMemoryUsage)
	}
	if bcs.Components.Retry.CPURequest == nil {
		bcs.Components.Retry.CPURequest = ptr.String(cpuRequest)
	}
	if bcs.Components.Retry.CPULimit == nil {
		bcs.Components.Retry.CPULimit = ptr.String(cpuLimit)
	}
	if bcs.Components.Retry.MemoryRequest == nil {
		bcs.Components.Retry.MemoryRequest = ptr.String(memoryRequest)
	}
	if bcs.Components.Retry.MemoryLimit == nil {
		bcs.Components.Retry.MemoryLimit = ptr.String(memoryLimit)
	}
	if bcs.Components.Retry.MinReplicas == nil {
		bcs.Components.Retry.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Retry.MaxReplicas == nil {
		bcs.Components.Retry.MaxReplicas = ptr.Int32(maxReplicas)
	}

	// Ingress defaults
	if bcs.Components.Ingress.AvgCPUUtilization == nil {
		bcs.Components.Ingress.AvgCPUUtilization = ptr.Int32(avgCPUUtilization)
	}
	if bcs.Components.Ingress.AvgMemoryUsage == nil {
		bcs.Components.Ingress.AvgMemoryUsage = ptr.String(avgMemoryUsageIngress)
	}
	if bcs.Components.Ingress.CPURequest == nil {
		bcs.Components.Ingress.CPURequest = ptr.String(cpuRequest)
	}
	if bcs.Components.Ingress.CPULimit == nil {
		bcs.Components.Ingress.CPULimit = ptr.String(cpuLimit)
	}
	if bcs.Components.Ingress.MemoryRequest == nil {
		bcs.Components.Ingress.MemoryRequest = ptr.String(memoryRequest)
	}
	if bcs.Components.Ingress.MemoryLimit == nil {
		bcs.Components.Ingress.MemoryLimit = ptr.String(memoryLimitIngress)
	}
	if bcs.Components.Ingress.MinReplicas == nil {
		bcs.Components.Ingress.MinReplicas = ptr.Int32(minReplicas)
	}
	if bcs.Components.Ingress.MaxReplicas == nil {
		bcs.Components.Ingress.MaxReplicas = ptr.Int32(maxReplicas)
	}
}
