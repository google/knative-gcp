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

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/knative-gcp/pkg/apis/duck"
)

func (current *CloudAuditLogsSource) Validate(ctx context.Context) *apis.FieldError {
	err := current.Spec.Validate(ctx).ViaField("spec")

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*CloudAuditLogsSource)
		err = err.Also(current.CheckImmutableFields(ctx, original))
	}
	return err
}

func (current *CloudAuditLogsSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, duckv1.Destination{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := current.Sink.Validate(ctx); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}

	// ServiceName [required]
	if current.ServiceName == "" {
		errs = errs.Also(apis.ErrMissingField("serviceName"))
	}
	// MethodName [required]
	if current.MethodName == "" {
		errs = errs.Also(apis.ErrMissingField("methodName"))
	}

	if err := duck.ValidateCredential(current.Secret, current.ServiceAccountName); err != nil {
		errs = errs.Also(err)
	}

	return errs
}

func (current *CloudAuditLogsSource) CheckImmutableFields(ctx context.Context, original *CloudAuditLogsSource) *apis.FieldError {
	if original == nil {
		return nil
	}

	var errs *apis.FieldError
	// Modification of Topic, Secret, ServiceAccountName, Project, ServiceName, MethodName and ResourceName are not allowed.
	// Everything else is mutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(CloudAuditLogsSourceSpec{},
			"Sink", "CloudEventOverrides")); diff != "" {
		errs = errs.Also(
			&apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec"},
				Details: diff,
			})
	}
	// Modification of AutoscalingClassAnnotations is not allowed.
	errs = duck.CheckImmutableAutoscalingClassAnnotations(&current.ObjectMeta, &original.ObjectMeta, errs)

	// Modification of non-empty cluster name annotation is not allowed.
	return duck.CheckImmutableClusterNameAnnotation(&current.ObjectMeta, &original.ObjectMeta, errs)
}
