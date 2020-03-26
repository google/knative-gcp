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

package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

// Validate validates the EventPolicyBinding.
func (pb *EventPolicyBinding) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	if err := pb.Spec.Validate(ctx, pb.Namespace); err != nil {
		errs = errs.Also(err.ViaField("spec"))
	}
	return errs
}

// CheckImmutableFields checks if any immutable fields are changed in an EventPolicyBinding.
func (pb *EventPolicyBinding) CheckImmutableFields(ctx context.Context, original *EventPolicyBinding) *apis.FieldError {
	if original == nil {
		return nil
	}
	var errs *apis.FieldError
	if err := CheckImmutableBindingObjectMeta(ctx, &pb.ObjectMeta, &original.ObjectMeta); err != nil {
		errs = errs.Also(err)
	}
	if err := pb.Spec.CheckImmutableFields(ctx, &original.Spec); err != nil {
		errs = errs.Also(err)
	}
	return errs
}
