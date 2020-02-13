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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func (current *CloudSchedulerSource) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *CloudSchedulerSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, duckv1.Destination{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := current.Sink.Validate(ctx); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}

	// Location [required]
	if current.Location == "" {
		errs = errs.Also(apis.ErrMissingField("location"))
	}

	// Schedule [required]
	if current.Schedule == "" {
		errs = errs.Also(apis.ErrMissingField("schedule"))
	}

	// Data [required]
	if current.Data == "" {
		errs = errs.Also(apis.ErrMissingField("data"))
	}

	if current.Secret != nil {
		if !equality.Semantic.DeepEqual(current.Secret, &corev1.SecretKeySelector{}) {
			err := validateSecret(current.Secret)
			if err != nil {
				errs = errs.Also(err.ViaField("secret"))
			}
		}
	}

	if current.PubSubSecret != nil {
		if !equality.Semantic.DeepEqual(current.PubSubSecret, &corev1.SecretKeySelector{}) {
			err := validateSecret(current.PubSubSecret)
			if err != nil {
				errs = errs.Also(err.ViaField("pubsubSecret"))
			}
		}
	}

	return errs
}

func (current *CloudSchedulerSource) CheckImmutableFields(ctx context.Context, original *CloudSchedulerSource) *apis.FieldError {
	if original == nil {
		return nil
	}
	// Modification of Location, Schedule, Data, Secret, PubSubSecret, Project are not allowed. Everything else is mutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(CloudSchedulerSourceSpec{},
			"Sink", "CloudEventOverrides")); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
