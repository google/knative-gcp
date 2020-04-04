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
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
)

func (current *CloudBuildSource) Validate(ctx context.Context) *apis.FieldError {
	errs := current.Spec.Validate(ctx).ViaField("spec")
	return duckv1alpha1.ValidateAutoscalingAnnotations(ctx, current.Annotations, errs)
}

func (current *CloudBuildSourceSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if current.Topic != nil && (!equality.Semantic.DeepEqual(*current.Topic, DefaultTopic)) {
		errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("topic should be omitted or must be equal to %q", DefaultTopic), "topic"))
	}

	// Sink [required]
	if equality.Semantic.DeepEqual(current.Sink, duckv1.Destination{}) {
		errs = errs.Also(apis.ErrMissingField("sink"))
	} else if err := current.Sink.Validate(ctx); err != nil {
		errs = errs.Also(err.ViaField("sink"))
	}

	if err := duckv1alpha1.ValidateCredential(current.Secret, current.GoogleServiceAccount); err != nil {
		errs = errs.Also(err)
	}

	return errs
}

func (current *CloudBuildSource) CheckImmutableFields(ctx context.Context, original *CloudBuildSource) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Modification of Topic, Secret and Project are not allowed. Everything else is mutable.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(CloudBuildSourceSpec{},
			"Sink", "CloudEventOverrides")); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}
