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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBroker_Validate(t *testing.T) {
	b := Broker{}
	if err := b.Validate(context.TODO()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestDeliverySpec_Validate(t *testing.T) {
	ds := &eventingduckv1beta1.DeliverySpec{}
	if err := ValidateDeliverySpec(context.TODO(), ds); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestDeadLetterSink_Validate(t *testing.T) {
	tests := []struct {
		name           string
		deadLetterSink *duckv1.Destination
		want           *apis.FieldError
	}{{
		name:           "invalid dead letter sink missing uri",
		deadLetterSink: &duckv1.Destination{},
		want:           apis.ErrMissingField("uri"),
	}, {
		name: "invalid dead letter sink uri scheme",
		deadLetterSink: &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "http",
				Host:   "test-topic-id",
				Path:   "/",
			},
		},
		want: apis.ErrInvalidValue("Dead letter sink URI scheme should be pubsub", "uri"),
	}, {
		name: "invalid empty dead letter topic id",
		deadLetterSink: &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "pubsub",
				Host:   "",
				Path:   "/",
			},
		},
		want: apis.ErrInvalidValue("Dead letter topic must not be empty", "uri"),
	}, {
		name: "invalid dead letter topic id too long",
		deadLetterSink: &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "pubsub",
				Host:   strings.Repeat("x", 256),
				Path:   "/",
			},
		},
		want: apis.ErrInvalidValue("Dead letter topic maximum length is 255 characters", "uri"),
	}, {
		name: "valid dead letter topic",
		deadLetterSink: &duckv1.Destination{
			URI: &apis.URL{
				Scheme: "pubsub",
				Host:   "test-topic-id",
				Path:   "/",
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateDeadLetterSink(context.Background(), test.deadLetterSink)
			//got := test.spec.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("ValidateDeadLetterSink (-want, +got) = %v", diff)
			}
		})
	}
}
