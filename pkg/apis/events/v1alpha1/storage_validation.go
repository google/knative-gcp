/*
Copyright 2019 Google LLC

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

func (current *Storage) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *StorageSpec) Validate(ctx context.Context) *apis.FieldError {
	// TODO
	var errs *apis.FieldError

	// Sink [required]
	if err := validateRef(current.Sink); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}
	return errs
}

func validateRef(ref corev1.ObjectReference) *apis.FieldError {
	var errs *apis.FieldError

	if equality.Semantic.DeepEqual(ref, corev1.ObjectReference{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if ref.APIVersion == "" {
		errs = errs.Also(apis.ErrMissingField("apiVersion"))
	}
	if ref.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}
	return errs
}
