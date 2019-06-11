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

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
)

func (current *PubSubSource) Validate(ctx context.Context) *apis.FieldError {
	return current.Spec.Validate(ctx).ViaField("spec")
}

func (current *PubSubSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	// Topic [required]
	if current.Topic == "" {
		errs = errs.Also(apis.ErrMissingField("topic"))
	}
	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, &corev1.ObjectReference{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := invalidateRef(current.Sink); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}
	// Transformer [optional]
	if !equality.Semantic.DeepEqual(current.Transformer, &corev1.ObjectReference{}) {
		if err := invalidateRef(current.Transformer); err != nil {
			errs = errs.Also(err.ViaField("transformer"))
		}
	}
	return nil
}

func invalidateRef(ref *corev1.ObjectReference) *apis.FieldError {
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

func (current *PubSubSource) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	original, ok := og.(*PubSubSource)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a PubSubSource"}
	}
	if original == nil {
		return nil
	}

	// TODO: revisit this.

	// All of the fields are immutable because the controller doesn't understand when it would need
	// to delete and create a new Receive Adapter with updated arguments. We could relax it slightly
	// to allow a nil Sink -> non-nil Sink, but I don't think it is needed yet.
	if diff := cmp.Diff(original.Spec, current.Spec); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
