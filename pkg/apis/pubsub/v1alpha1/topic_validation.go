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
	"fmt"

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

func (current *Topic) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	original, ok := og.(*Topic)
	if !ok {
		return &apis.FieldError{Message: "The provided original was not a Topic"}
	}
	if original == nil {
		return nil
	}

	var errs *apis.FieldError

	// Topic is immutable.
	if original.Spec.Topic != current.Spec.Topic {
		errs = errs.Also(
			&apis.FieldError{
				Message: "Immutable field changed",
				Paths:   []string{"spec", "topic"},
				Details: fmt.Sprintf("was %q, now %q", original.Spec.Topic, current.Spec.Topic),
			})
	}
	return errs
}
