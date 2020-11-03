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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/knative-gcp/pkg/apis/configs/brokerdelivery"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	clusterDefaultedBackoffDelay        = "PT1S"
	clusterDefaultedBackoffPolicy       = eventingduckv1beta1.BackoffPolicyExponential
	clusterDefaultedRetry         int32 = 6
	nsDefaultedBackoffDelay             = "PT2S"
	nsDefaultedBackoffPolicy            = eventingduckv1beta1.BackoffPolicyLinear
	nsDefaultedRetry              int32 = 10
	customRetry                   int32 = 5

	defaultConfig = &brokerdelivery.Config{
		BrokerDeliverySpecDefaults: &brokerdelivery.Defaults{
			// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
			// Namespace is the key, the value is the KReference to the config.
			NamespaceDefaults: map[string]brokerdelivery.ScopedDefaults{
				"mynamespace": {
					DeliverySpec: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &nsDefaultedBackoffDelay,
						BackoffPolicy: &nsDefaultedBackoffPolicy,
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "ns-default-dead-letter-topic-id",
							},
						},
						Retry: &nsDefaultedRetry,
					},
				},
				"mynamespace2": {
					DeliverySpec: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "ns-default-dead-letter-topic-id",
							},
						},
						Retry: &nsDefaultedRetry,
					},
				},
				"mynamespace3": {
					DeliverySpec: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &nsDefaultedBackoffDelay,
						BackoffPolicy: &nsDefaultedBackoffPolicy,
					},
				},
				"mynamespace4": {
					DeliverySpec: &eventingduckv1beta1.DeliverySpec{
						Retry: &nsDefaultedRetry,
					},
				},
			},
			ClusterDefaults: brokerdelivery.ScopedDefaults{
				DeliverySpec: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay:  &clusterDefaultedBackoffDelay,
					BackoffPolicy: &clusterDefaultedBackoffPolicy,
					DeadLetterSink: &duckv1.Destination{
						URI: &apis.URL{
							Scheme: "pubsub",
							Host:   "cluster-default-dead-letter-topic-id",
						},
					},
					Retry: &clusterDefaultedRetry,
				},
			},
		},
	}
)

func TestBroker_SetDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Broker
		expected Broker
	}{
		"default everything from cluster": {
			expected: Broker{
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &clusterDefaultedBackoffDelay,
						BackoffPolicy: &clusterDefaultedBackoffPolicy,
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "cluster-default-dead-letter-topic-id",
							},
						},
						Retry: &clusterDefaultedRetry,
					},
				},
			},
		},
		"default backoff policy from cluster": {
			expected: Broker{
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &clusterDefaultedBackoffDelay,
						BackoffPolicy: &clusterDefaultedBackoffPolicy,
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "custom-topic-id",
							},
						},
						Retry: &customRetry,
					},
				},
			},
			initial: Broker{
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "custom-topic-id",
							},
						},
						Retry: &customRetry,
					},
				},
			},
		},
		"default everything from namespace": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace",
				},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &nsDefaultedBackoffDelay,
						BackoffPolicy: &nsDefaultedBackoffPolicy,
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "ns-default-dead-letter-topic-id",
							},
						},
						Retry: &nsDefaultedRetry,
					},
				},
			},
		},
		"default namespace dead letter policy": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace2"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace2",
				},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "ns-default-dead-letter-topic-id",
							},
						},
						Retry: &nsDefaultedRetry,
					},
				},
			},
		},
		"default namespace backoff policy": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace3"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace3",
				},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						BackoffDelay:  &nsDefaultedBackoffDelay,
						BackoffPolicy: &nsDefaultedBackoffPolicy,
					},
				},
			},
		},
		"no dead letter sink, do not set retry": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace4"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace4",
				},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{},
				},
			},
		},
		"default namespace retry": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace4"},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "custom-topic-id",
							},
						},
					},
				},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace4",
				},
				Spec: eventingv1beta1.BrokerSpec{
					Delivery: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: &apis.URL{
								Scheme: "pubsub",
								Host:   "custom-topic-id",
							},
						},
						Retry: &nsDefaultedRetry,
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(brokerdelivery.ToContext(context.Background(), defaultConfig))
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
