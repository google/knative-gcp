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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"knative.dev/pkg/apis"
)

func (current *CloudAuditLogsSource) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *CloudAuditLogsSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	// ServiceName [required]
	if current.ServiceName == "" {
		errs = errs.Also(apis.ErrMissingField("serviceName"))
	}
	// MethodName [required]
	if current.MethodName == "" {
		errs = errs.Also(apis.ErrMissingField("methodName"))
	}

	return errs
}

func (current *CloudAuditLogsSource) CheckImmutableFields(ctx context.Context, original *CloudAuditLogsSource) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Modification of Topic, Secret, Project, ServiceName, MethodName, and ResourceName are not allowed. Everything else is mutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(CloudAuditLogsSourceSpec{},
			"Sink", "CloudEventOverrides")); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
