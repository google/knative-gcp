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

package v1beta1

import (
	"context"
	"fmt"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"

	"github.com/google/knative-gcp/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

func (c *Channel) Validate(ctx context.Context) *apis.FieldError {
	err := c.Spec.Validate(ctx).ViaField("spec")

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Channel)
		err = err.Also(c.CheckImmutableFields(ctx, original))
	}
	return err
}

func (cs *ChannelSpec) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if cs.SubscribableSpec != nil {
		for i, subscriber := range cs.SubscribableSpec.Subscribers {
			if subscriber.ReplyURI == nil && subscriber.SubscriberURI == nil {
				fe := apis.ErrMissingField("replyURI", "subscriberURI")
				fe.Details = "expected at least one of, got none"
				errs = errs.Also(fe.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("subscribable"))
			}
			deliveryErr := brokerv1beta1.ValidateDeliverySpec(ctx, subscriber.Delivery)
			if deliveryErr != nil {
				errs = errs.Also(deliveryErr.ViaField(fmt.Sprintf("subscriber[%d]", i)).ViaField("delivery"))
			}
		}
	}

	return errs
}

func (current *Channel) CheckImmutableFields(ctx context.Context, original *Channel) *apis.FieldError {
	if original == nil {
		return nil
	}

	var errs *apis.FieldError

	// Modification of AutoscalingClassAnnotations is not allowed.
	errs = duck.CheckImmutableAutoscalingClassAnnotations(&current.ObjectMeta, &original.ObjectMeta, errs)

	// Modification of non-empty cluster name annotation is not allowed.
	return duck.CheckImmutableClusterNameAnnotation(&current.ObjectMeta, &original.ObjectMeta, errs)
}
