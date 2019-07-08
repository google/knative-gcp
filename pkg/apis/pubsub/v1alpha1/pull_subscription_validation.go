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
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

const (
	minRetentionDuration = 10 * time.Second   // 10 seconds.
	maxRetentionDuration = 7 * 24 * time.Hour // 7 days.
)

func (current *PullSubscription) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *PullSubscriptionSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	// Topic [required]
	if current.Topic == "" {
		errs = errs.Also(apis.ErrMissingField("topic"))
	}
	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, &corev1.ObjectReference{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := validateRef(current.Sink); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}
	// Transformer [optional]
	if !equality.Semantic.DeepEqual(current.Transformer, &corev1.ObjectReference{}) {
		if err := validateRef(current.Transformer); err != nil {
			errs = errs.Also(err.ViaField("transformer"))
		}
	}

	if current.RetentionDuration != nil {
		// If set, RetentionDuration Cannot be longer than 7 days or shorter than 10 minutes.
		if *current.RetentionDuration < minRetentionDuration || *current.RetentionDuration > maxRetentionDuration {
			errs = errs.Also(apis.ErrInvalidValue(current.RetentionDuration, "retentionDuration"))
		}
	}
	return nil
}

func validateRef(ref *corev1.ObjectReference) *apis.FieldError {
	// nil check.
	if ref == nil {
		return apis.ErrMissingField(apis.CurrentField)
	}
	// Check the object.
	var errs *apis.FieldError
	// Required Fields
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

func (current *PullSubscription) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	original, ok := og.(*PullSubscription)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a PullSubscription"}
	}
	if original == nil {
		return nil
	}

	// Modification of Sink and Transform allowed. Everything else is immutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(PullSubscriptionSpec{}, "Sink", "Transformer")); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
