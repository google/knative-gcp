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

package resource

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// MultiplyQuantity multiplies resource quantity by the specified value and returns it in the original format
func MultiplyQuantity(quantity resource.Quantity, val float64) *resource.Quantity {
	if val < 0 {
		panic("Resource quantity multiplication factor should not be negative.")
	}
	rawResult := int64(float64(quantity.Value()) * val)
	return resource.NewQuantity(rawResult, quantity.Format)
}

// BuildResourceRequirements constructs the core resource requirements structure based on specified resource requests and limits
func BuildResourceRequirements(cpuRequest, cpuLimit, memoryRequest, memoryLimit string) corev1.ResourceRequirements {
	resourceRequirements := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if len(cpuRequest) > 0 {
		cpuRequestQuantity, err := resource.ParseQuantity(cpuRequest)
		if err == nil {
			resourceRequirements.Requests[corev1.ResourceCPU] = cpuRequestQuantity
		}
	}
	if len(cpuLimit) > 0 {
		cpuLimitQuantity, err := resource.ParseQuantity(cpuLimit)
		if err == nil {
			resourceRequirements.Limits[corev1.ResourceCPU] = cpuLimitQuantity
		}
	}
	if len(memoryRequest) > 0 {
		memoryRequestQuantity, err := resource.ParseQuantity(memoryRequest)
		if err == nil {
			resourceRequirements.Requests[corev1.ResourceMemory] = memoryRequestQuantity
		}
	}
	if len(memoryLimit) > 0 {
		memoryLimitQuantity, err := resource.ParseQuantity(memoryLimit)
		if err == nil {
			resourceRequirements.Limits[corev1.ResourceMemory] = memoryLimitQuantity
		}
	}

	return resourceRequirements
}

func IsValidQuantity(quantityReference *string) bool {
	if quantityReference == nil {
		return true
	}
	return IsValidQuantityValue(*quantityReference)
}

func IsValidQuantityValue(value string) bool {
	if len(value) == 0 {
		return true
	}
	_, parsingErr := resource.ParseQuantity(value)
	if parsingErr != nil {
		return false
	}
	return true
}
