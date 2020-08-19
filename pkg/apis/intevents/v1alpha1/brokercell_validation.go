/*
Copyright 2020 The Knative Authors

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
	"fmt"

	resourceutil "github.com/google/knative-gcp/pkg/utils/resource"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"
)

// Validate verifies that the BrokerCell is valid.
func (bc *BrokerCell) Validate(ctx context.Context) *apis.FieldError {
	fieldErrors := bc.Spec.Validate(ctx).ViaField("spec")
	return fieldErrors
}

func (bcs *BrokerCellSpec) Validate(ctx context.Context) *apis.FieldError {
	var fieldErrors *apis.FieldError
	if bcs.Components.Fanout != nil {
		fieldErrors = bcs.Components.Fanout.ValidateResourceRequirementSpecification(fieldErrors, "components.fanout")
	}
	if bcs.Components.Ingress != nil {
		fieldErrors = bcs.Components.Ingress.ValidateResourceRequirementSpecification(fieldErrors, "components.ingress")
	}
	if bcs.Components.Retry != nil {
		fieldErrors = bcs.Components.Retry.ValidateResourceRequirementSpecification(fieldErrors, "components.retry")
	}
	return fieldErrors
}

func (componentParams *ComponentParameters) ValidateResourceRequirementSpecification(fieldErrors *apis.FieldError, componentPath string) *apis.FieldError {
	fieldErrors = componentParams.ValidateQuantityFormats(fieldErrors, componentPath)
	fieldErrors = componentParams.ValidateResourceSpecification(fieldErrors, componentPath)
	fieldErrors = componentParams.ValidateAutoscalingSpecification(fieldErrors, componentPath)
	return fieldErrors
}

func (componentParams *ComponentParameters) ValidateResourceSpecification(fieldErrors *apis.FieldError, componentPath string) *apis.FieldError {
	// Make sure the CPU limit is not lower than what's requested (when both are set)
	if componentParams.Resources.Requests.CPU != "" && componentParams.Resources.Limits.CPU != "" {
		cpuRequestQuantity, errRequest := resource.ParseQuantity(componentParams.Resources.Requests.CPU)
		cpuLimitQuantity, errLimit := resource.ParseQuantity(componentParams.Resources.Limits.CPU)
		if errRequest == nil && errLimit == nil && cpuLimitQuantity.Cmp(cpuRequestQuantity) < 0 {
			invalidValueError := apis.ErrInvalidValue(componentParams.Resources.Requests.CPU, "cpu").ViaField(fmt.Sprintf("%s.resources.requests", componentPath))
			invalidValueError.Details = "Resource request should not exceed the resource limit"
			fieldErrors = fieldErrors.Also(invalidValueError)
		}
	}
	// Make sure the memory limit is not lower than what's requested (when both are set)
	if componentParams.Resources.Requests.Memory != "" && componentParams.Resources.Limits.Memory != "" {
		memoryRequestQuantity, errRequest := resource.ParseQuantity(componentParams.Resources.Requests.Memory)
		memoryLimitQuantity, errLimit := resource.ParseQuantity(componentParams.Resources.Limits.Memory)
		if errRequest == nil && errLimit == nil && memoryLimitQuantity.Cmp(memoryRequestQuantity) < 0 {
			invalidValueError := apis.ErrInvalidValue(componentParams.Resources.Requests.Memory, "memory").ViaField(fmt.Sprintf("%s.resources.requests", componentPath))
			invalidValueError.Details = "Resource request should not exceed the resource limit"
			fieldErrors = fieldErrors.Also(invalidValueError)
		}
	}
	return fieldErrors
}

func (componentParams *ComponentParameters) ValidateAutoscalingSpecification(fieldErrors *apis.FieldError, componentPath string) *apis.FieldError {
	// AvgMemoryUsage should not exceed the memory limit (when both are set)
	if componentParams.AvgMemoryUsage != nil && *componentParams.AvgMemoryUsage != "" && componentParams.Resources.Limits.Memory != "" {
		avgMemoryUsageQuantity, errAvgMemoryUsage := resource.ParseQuantity(*componentParams.AvgMemoryUsage)
		memoryLimitQuantity, errLimit := resource.ParseQuantity(componentParams.Resources.Limits.Memory)
		if componentParams.AvgMemoryUsage != nil && errLimit == nil && errAvgMemoryUsage == nil {
			if memoryLimitQuantity.Cmp(avgMemoryUsageQuantity) < 0 {
				invalidValueError := apis.ErrInvalidValue(*componentParams.AvgMemoryUsage, fmt.Sprintf("%s.AvgMemoryUsage", componentPath))
				invalidValueError.Details = "AvgMemoryUsage should not exceed the memory limit"
				fieldErrors = fieldErrors.Also(invalidValueError)
			}
		}
	}
	// At least one of the autoscaling metrics should be specified
	isAvgMemoryUsageSpecified := componentParams.AvgMemoryUsage != nil && *componentParams.AvgMemoryUsage != ""
	if componentParams.AvgCPUUtilization == nil && !isAvgMemoryUsageSpecified {
		invalidValueError := apis.ErrInvalidValue(nil, componentPath)
		invalidValueError.Details = "At least one of the autoscaling metrics (AvgCPUUtilization, AvgMemoryUsage) should be specified"
		fieldErrors = fieldErrors.Also(invalidValueError)
	}
	if componentParams.MinReplicas != nil && componentParams.MaxReplicas != nil && *componentParams.MinReplicas > *componentParams.MaxReplicas {
		invalidValueError := apis.ErrInvalidValue(*componentParams.MinReplicas, fmt.Sprintf("%s.MinReplicas", componentPath))
		invalidValueError.Details = "MinReplicas value can not exceed the value of MaxReplicas"
		fieldErrors = fieldErrors.Also(invalidValueError)
	}
	return fieldErrors
}

func (componentParams *ComponentParameters) ValidateQuantityFormats(fieldErrors *apis.FieldError, componentPath string) *apis.FieldError {
	fieldsToValidate := []struct {
		fieldName string
		value     *string
	}{
		{
			fieldName: fmt.Sprintf("%s.resources.requests.cpu", componentPath),
			value:     &componentParams.Resources.Requests.CPU,
		},
		{
			fieldName: fmt.Sprintf("%s.resources.requests.memory", componentPath),
			value:     &componentParams.Resources.Requests.Memory,
		},
		{
			fieldName: fmt.Sprintf("%s.resources.limits.cpu", componentPath),
			value:     &componentParams.Resources.Limits.CPU,
		},
		{
			fieldName: fmt.Sprintf("%s.resources.limits.memory", componentPath),
			value:     &componentParams.Resources.Limits.Memory,
		},
		{
			fieldName: fmt.Sprintf("%s.AvgMemoryUsage", componentPath),
			value:     componentParams.AvgMemoryUsage,
		},
	}

	for _, validation := range fieldsToValidate {
		if !resourceutil.IsValidQuantity(validation.value) {
			unexpectedFormatError := apis.ErrInvalidValue(*validation.value, validation.fieldName)
			unexpectedFormatError.Details = "The quantity is specified in an unexpected format"
			fieldErrors = fieldErrors.Also(unexpectedFormatError)
		}
	}
	return fieldErrors
}
