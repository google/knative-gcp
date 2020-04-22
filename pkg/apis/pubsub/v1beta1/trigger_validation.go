/*
Copyright 2020 Google LLC.

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

package v1beta1

import (
	"context"

	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	RequiredKeys = map[string][]string{
		TriggerAuditLogs: {"ServiceName", "MethodName"},
	}
	ValidKeys = map[string][]string{
		TriggerAuditLogs: {"ServiceName", "MethodName", "ResourceName"},
	}
)

func (current *Trigger) Validate(ctx context.Context) *apis.FieldError {
	errs := current.Spec.Validate(ctx).ViaField("spec")
	return duckv1beta1.ValidateAutoscalingAnnotations(ctx, current.Annotations, errs)
}

func (current *TriggerSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// SourceType [required]
	if current.SourceType == "" {
		errs = errs.Also(apis.ErrMissingField("sourceType"))
	}

	// Validation for filter
	for _, required := range RequiredKeys[current.SourceType] {
		if _, ok := current.Filters[required]; !ok {
			// TODO(nlopezgi): need a ErrMissingKeyValue method here
			errs = errs.Also(apis.ErrMissingField("filter." + required))
		}
	}
	for key := range current.Filters {
		isValid := false
		for _, valid := range ValidKeys[current.SourceType] {
			if key == valid {
				isValid = true
			}
		}
		if !isValid {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "filters"))
		}
	}

	if err := duckv1beta1.ValidateCredential(current.Secret, current.GoogleServiceAccount); err != nil {
		errs = errs.Also(err)
	}

	return errs
}

func (current *Trigger) CheckImmutableFields(ctx context.Context, original *Trigger) *apis.FieldError {
	if original == nil {
		return nil
	}
	// Modification of Secret, ServiceAccount, Project, SourceType and Filters are not allowed. Everything else is mutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(TriggerSpec{})); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
