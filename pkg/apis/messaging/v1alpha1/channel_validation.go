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

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

func (c *Channel) Validate(ctx context.Context) *apis.FieldError {
	return c.Spec.Validate(ctx).ViaField("spec")
}

func (cs *ChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if cs.Subscribable != nil {
		for i, subscriber := range cs.Subscribable.Subscribers {
			if subscriber.ReplyURI == nil && subscriber.SubscriberURI == nil {
				fe := apis.ErrMissingField("replyURI", "subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
			}
		}
	}

	if err := duckv1alpha1.ValidateCredential(cs.Secret, cs.GoogleServiceAccount); err != nil {
		errs = errs.Also(err)
	}

	return errs
}

func (current *Channel) CheckImmutableFields(ctx context.Context, original *Channel) *apis.FieldError {
	if original == nil {
		return nil
	}

	// Modification of TopicID is not allowed.
	if original.Status.TopicID != "" {
		if diff := cmp.Diff(original.Status.TopicID, current.Status.TopicID); diff != "" {
			return &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"status", "topicId"},
				Details: diff,
			}
		}
	}

	if diff := cmp.Diff(original.Spec.GoogleServiceAccount, current.Spec.GoogleServiceAccount); diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"status", "serviceAccount"},
			Details: diff,
		}
	}

	return nil
}
