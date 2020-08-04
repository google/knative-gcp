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

package v1beta1

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	// DefaultBackoffDelay is the default backoff delay used in the backoff retry policy
	// for the Broker delivery spec.
	DefaultBackoffDelay = "PT1S"
	// DefaultBackoffPolicy is the default backoff policy type used in the backoff retry
	// policy for the Broker delivery spec.
	DefaultBackoffPolicy = eventingduckv1beta1.BackoffPolicyExponential
	// DefaultRetry is the default number of maximum delivery attempts for unacked messages
	// before they are sent to a dead letter topic in the Broker delivery spec, in
	// case a dead letter topic is specified. Without a dead letter topic specified,
	// the retry count is infinite.
	DefaultRetry int32 = 6
)

// SetDefaults sets the default field values for a Broker.
func (b *Broker) SetDefaults(ctx context.Context) {
	// Set the default delivery spec.
	if b.Spec.Delivery == nil {
		b.Spec.Delivery = &eventingduckv1beta1.DeliverySpec{}
	}
	if b.Spec.Delivery.BackoffPolicy == nil &&
		b.Spec.Delivery.BackoffDelay == nil {
		b.Spec.Delivery.BackoffPolicy = &DefaultBackoffPolicy
		b.Spec.Delivery.BackoffDelay = &DefaultBackoffDelay
	}
	if b.Spec.Delivery.Retry == nil &&
		(b.Spec.Delivery.DeadLetterSink != nil && !equality.Semantic.DeepEqual(b.Spec.Delivery.DeadLetterSink, &duckv1.Destination{})) {
		b.Spec.Delivery.Retry = &DefaultRetry
	}
	// Besides this, the eventing webhook will add the usual defaults.
}
