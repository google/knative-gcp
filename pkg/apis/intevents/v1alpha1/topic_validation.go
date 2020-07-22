/*
Copyright 2019 The Knative Authors

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
	"github.com/google/knative-gcp/pkg/apis/duck"

	"knative.dev/pkg/apis"
)

func (t *Topic) Validate(ctx context.Context) *apis.FieldError {
	return t.Spec.Validate(ctx).ViaField("spec")
}

func (ts *TopicSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if ts.Topic == "" {
		errs = errs.Also(
			apis.ErrMissingField("topic"),
		)
	}

	switch ts.PropagationPolicy {
	case TopicPolicyCreateDelete, TopicPolicyCreateNoDelete, TopicPolicyNoCreateNoDelete:
	// Valid value.

	default:
		errs = errs.Also(
			apis.ErrInvalidValue(ts.PropagationPolicy, "propagationPolicy"),
		)
	}

	return errs
}

func (current *Topic) CheckImmutableFields(ctx context.Context, original *Topic) *apis.FieldError {
	if original == nil {
		return nil
	}

	var errs *apis.FieldError
	// Modification of Topic, Secret, ServiceAccountName, PropagationPolicy, EnablePublisher and Project are not allowed.
	if diff := cmp.Diff(original.Spec, current.Spec,
		cmpopts.IgnoreFields(TopicSpec{})); diff != "" {
		errs = errs.Also(&apis.FieldError{
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
