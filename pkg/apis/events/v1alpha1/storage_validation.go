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
	apisv1alpha1 "knative.dev/pkg/apis/v1alpha1"
)

func (current *Storage) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *StorageSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, apisv1alpha1.Destination{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := current.Sink.Validate(ctx); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}

	// Bucket [required]
	if current.Bucket == "" {
		errs = errs.Also(apis.ErrMissingField("bucket"))
	}

	if !equality.Semantic.DeepEqual(&current.GCSSecret, &corev1.SecretKeySelector{}) {
		err := validateSecret(&current.GCSSecret)
		if err != nil {
			errs = errs.Also(err.ViaField("gcsSecret"))
		}
	}

	if current.PullSubscriptionSecret != nil {
		err := validateSecret(current.PullSubscriptionSecret)
		if err != nil {
			errs = errs.Also(err.ViaField("pullSubscriptionSecret"))
		}
	}

	return errs
}

func validateSecret(secret *corev1.SecretKeySelector) *apis.FieldError {
	var errs *apis.FieldError
	if secret.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	if secret.Key == "" {
		errs = errs.Also(apis.ErrMissingField("key"))
	}
	return errs
}
