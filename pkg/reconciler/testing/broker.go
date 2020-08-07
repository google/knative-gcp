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

package testing

import (
	"context"
	"time"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*brokerv1beta1.Broker)

// NewBroker creates a Broker with BrokerOptions. The Broker has the
// brokerv1beta1 broker class by default.
func NewBroker(name, namespace string, o ...BrokerOption) *brokerv1beta1.Broker {
	b := &brokerv1beta1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				eventingv1beta1.BrokerClassAnnotationKey: brokerv1beta1.BrokerClass,
			},
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(b)
	}
	return b
}

// WithInitBrokerConditions initializes the Broker's conditions.
func WithInitBrokerConditions(b *brokerv1beta1.Broker) {
	b.Status.InitializeConditions()
}

func WithBrokerFinalizers(finalizers ...string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Finalizers = finalizers
	}
}

func WithBrokerResourceVersion(rv string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.ResourceVersion = rv
	}
}

func WithBrokerUID(uid string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.UID = types.UID(uid)
	}
}

func WithBrokerGeneration(gen int64) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Generation = gen
	}
}

func WithBrokerStatusObservedGeneration(gen int64) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Status.ObservedGeneration = gen
	}
}

func WithBrokerDeletionTimestamp(b *brokerv1beta1.Broker) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	b.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithBrokerAddress sets the Broker's address.
func WithBrokerAddress(address string) BrokerOption {
	return WithBrokerAddressURI(&apis.URL{
		Scheme: "http",
		Host:   address,
	})
}

// WithBrokerAddressURI sets the Broker's address as URI.
func WithBrokerAddressURI(uri *apis.URL) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Status.SetAddress(uri)
	}
}

func WithBrokerReady(address string) BrokerOption {
	return WithBrokerReadyURI(&apis.URL{
		Scheme: "http",
		Host:   address,
	})
}

// WithBrokerReadyURI is a convenience function that sets all ready conditions to
// true.
func WithBrokerReadyURI(address *apis.URL) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		WithBrokerBrokerCellReady(b)
		WithBrokerSubscriptionReady(b)
		WithBrokerTopicReady(b)
		WithBrokerAddressURI(address)(b)
	}
}

func WithBrokerBrokerCellFailed(reason, msg string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Status.MarkBrokerCellFailed(reason, msg)
	}
}

func WithBrokerBrokerCellUnknown(reason, msg string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Status.MarkBrokerCellUnknown(reason, msg)
	}
}

func WithBrokerBrokerCellReady(b *brokerv1beta1.Broker) {
	b.Status.MarkBrokerCellReady()
}

func WithBrokerSubscriptionReady(b *brokerv1beta1.Broker) {
	b.Status.MarkSubscriptionReady()
}

func WithBrokerTopicReady(b *brokerv1beta1.Broker) {
	b.Status.MarkTopicReady()
}

func WithBrokerClass(bc string) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[eventingv1beta1.BrokerClassAnnotationKey] = bc
		b.SetAnnotations(annotations)
	}
}

func WithBrokerSetDefaults(b *brokerv1beta1.Broker) {
	b.SetDefaults(context.Background())
}

// WithBrokerDeliverySpec sets the Broker's delivery spec.
func WithBrokerDeliverySpec(deliverySpec *eventingduckv1beta1.DeliverySpec) BrokerOption {
	return func(b *brokerv1beta1.Broker) {
		b.Spec.Delivery = deliverySpec
	}
}
