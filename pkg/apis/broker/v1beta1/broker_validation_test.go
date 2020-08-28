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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBroker_Validate(t *testing.T) {
	bop := eventingduckv1beta1.BackoffPolicyExponential
	bod := "PT1S"
	tests := []struct {
		name   string
		broker Broker
		want   *apis.FieldError
	}{{
		name: "no error on missing delivery spec",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{},
		},
	}, {
		name: "missing backoff policy",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay: &bod,
				},
			},
		},
		want: apis.ErrMissingField("spec.delivery.backoffPolicy"),
	}, {
		name: "missing backoff delay",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffPolicy: &bop,
				},
			},
		},
		want: apis.ErrMissingField("spec.delivery.backoffDelay"),
	}, {
		name: "invalid dead letter sink missing uri",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:   &bod,
					BackoffPolicy:  &bop,
					DeadLetterSink: &duckv1.Destination{},
				},
			},
		},
		want: apis.ErrMissingField("spec.delivery.deadLetterSink.uri"),
	}, {
		name: "invalid dead letter sink uri scheme",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:  &bod,
					BackoffPolicy: &bop,
					DeadLetterSink: &duckv1.Destination{
						URI: &apis.URL{
							Scheme: "http",
							Host:   "test-topic-id",
						},
					},
				},
			},
		},
		want: apis.ErrInvalidValue("Dead letter sink URI scheme should be pubsub", "spec.delivery.deadLetterSink.uri"),
	}, {
		name: "invalid empty dead letter topic id",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:  &bod,
					BackoffPolicy: &bop,
					DeadLetterSink: &duckv1.Destination{
						URI: &apis.URL{
							Scheme: "pubsub",
							Host:   "",
						},
					},
				},
			},
		},
		want: apis.ErrInvalidValue("Dead letter topic must not be empty", "spec.delivery.deadLetterSink.uri"),
	}, {
		name: "invalid dead letter topic id too long",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:  &bod,
					BackoffPolicy: &bop,
					DeadLetterSink: &duckv1.Destination{
						URI: &apis.URL{
							Scheme: "pubsub",
							Host:   strings.Repeat("x", 256),
						},
					},
				},
			},
		},
		want: apis.ErrInvalidValue("Dead letter topic maximum length is 255 characters", "spec.delivery.deadLetterSink.uri"),
	}, {
		name: "valid dead letter topic",
		broker: Broker{
			Spec: v1beta1.BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:  &bod,
					BackoffPolicy: &bop,
					DeadLetterSink: &duckv1.Destination{
						URI: &apis.URL{
							Scheme: "pubsub",
							Host:   "test-topic-id",
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.broker.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("TestBroker_Validate (-want, +got) = %v", diff)
			}
		})
	}
}
