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

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
)

var (
	// backoffDelay is the default backoff delay used in the backoff retry policy
	// for the Broker delivery spec.
	backoffDelay = "PT1S"
	// backoffPolicy is the default backoff policy type used in the backoff retry
	// policy for the Broker delivery spec.
	backoffPolicy = eventingduckv1beta1.BackoffPolicyExponential
	// retry is the default number of maximum delivery attempts for unacked messages
	// before they are sent to a dead letter topic in the Broker delivery spec.
	retry int32 = 6
)

// SetDefaults sets the default field values for a Broker.
func (b *Broker) SetDefaults(ctx context.Context) {
	// Default delivery spec fields.
	b.Spec.Delivery = &eventingduckv1beta1.DeliverySpec{
		BackoffDelay:  &backoffDelay,
		BackoffPolicy: &backoffPolicy,
		Retry:         &retry,
	}
	// Besides this, the eventing webhook will add the usual defaults.
}
