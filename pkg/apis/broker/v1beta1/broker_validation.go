/*
Copyright 2020 The Knative Authors

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

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// Validate verifies that the Broker is valid.
func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	// We validate the GCP Broker's delivery spec. The eventing webhook will run
	// the other usual validations.
	if b.Spec.Delivery == nil {
		return nil
	}
	withNS := apis.AllowDifferentNamespace(apis.WithinParent(ctx, b.ObjectMeta))
	return ValidateDeliverySpec(withNS, b.Spec.Delivery).ViaField("spec", "delivery")
}

func ValidateDeliverySpec(ctx context.Context, spec *eventingduckv1beta1.DeliverySpec) *apis.FieldError {
	var errs *apis.FieldError
	if spec.BackoffDelay == nil {
		errs = errs.Also(apis.ErrMissingField("backoffDelay"))
	}
	if spec.BackoffPolicy == nil {
		errs = errs.Also(apis.ErrMissingField("backoffPolicy"))
	}
	return errs.Also(ValidateDeadLetterSink(ctx, spec.DeadLetterSink).ViaField("deadLetterSink"))
}

func ValidateDeadLetterSink(ctx context.Context, sink *duckv1.Destination) *apis.FieldError {
	if sink == nil {
		return nil
	}
	if sink.URI == nil {
		return apis.ErrMissingField("uri")
	}
	if scheme := sink.URI.Scheme; scheme != "pubsub" {
		return apis.ErrInvalidValue("Dead letter sink URI scheme should be pubsub", "uri")
	}
	topicID := sink.URI.Host
	if topicID == "" {
		return apis.ErrInvalidValue("Dead letter topic must not be empty", "uri")
	}
	if len(topicID) > 255 {
		return apis.ErrInvalidValue("Dead letter topic maximum length is 255 characters", "uri")
	}
	return nil
}
