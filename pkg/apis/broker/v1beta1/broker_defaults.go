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
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/apis/configs/brokerdelivery"
)

// SetDefaults sets the default field values for a Broker.
func (b *Broker) SetDefaults(ctx context.Context) {
	// Apply the default Broker delivery settings from the context.
	withNS := apis.WithinParent(ctx, b.ObjectMeta)
	deliverySpecDefaults := brokerdelivery.FromContextOrDefaults(withNS).BrokerDeliverySpecDefaults
	if deliverySpecDefaults == nil {
		logging.FromContext(ctx).Error("Failed to get the BrokerDeliverySpecDefaults")
		return
	}
	// Set the default delivery spec.
	if b.Spec.Delivery == nil {
		b.Spec.Delivery = &eventingduckv1beta1.DeliverySpec{}
	}
	ns := apis.ParentMeta(withNS).Namespace
	if b.Spec.Delivery.BackoffPolicy == nil || b.Spec.Delivery.BackoffDelay == nil {
		// Set both defaults if one of the backoff delay or backoff policy are not specified.
		b.Spec.Delivery.BackoffPolicy = deliverySpecDefaults.BackoffPolicy(ns)
		b.Spec.Delivery.BackoffDelay = deliverySpecDefaults.BackoffDelay(ns)
	}
	if b.Spec.Delivery.DeadLetterSink == nil {
		b.Spec.Delivery.DeadLetterSink = deliverySpecDefaults.DeadLetterSink(ns)
	}
	if b.Spec.Delivery.Retry == nil && b.Spec.Delivery.DeadLetterSink != nil {
		// Only set the retry count if a dead letter sink is specified.
		b.Spec.Delivery.Retry = deliverySpecDefaults.Retry(ns)
	}
	// Besides this, the eventing webhook will add the usual defaults.
}
