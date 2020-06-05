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

package resources

import (
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	"github.com/google/knative-gcp/pkg/utils/naming"
)

// For reference, the minimum number of characters available for a name
// is 146. However, any name longer than 146 will be truncated and suffixed
// with a 32-char hash, making its max length 114 chars.
//
// pubsub resource name max length: 255 chars
// Namespace max length: 63 chars
// broker name max length: 253 chars
// trigger name max length: 253 chars
// uid length: 36 chars
// prefix + separators: 10 chars
// 255 - 10 - 63 - 36 = 146

// GenerateDecouplingTopicName generates a deterministic topic name for a
// Broker. If the topic name would be longer than allowed by PubSub, the
// Broker name is truncated to fit.
func GenerateDecouplingTopicName(b *brokerv1beta1.Broker) string {
	return naming.TruncatedPubsubResourceName("cre-bkr", b.Namespace, b.Name, b.UID)
}

// GenerateDecouplingSubscriptionName generates a deterministic subscription
// name for a Broker. If the subscription name would be longer than allowed by
// PubSub, the Broker name is truncated to fit.
func GenerateDecouplingSubscriptionName(b *brokerv1beta1.Broker) string {
	return naming.TruncatedPubsubResourceName("cre-bkr", b.Namespace, b.Name, b.UID)
}

// GenerateRetryTopicName generates a deterministic topic name for a Trigger.
// If the topic name would be longer than allowed by PubSub, the Trigger name is
// truncated to fit.
func GenerateRetryTopicName(t *brokerv1beta1.Trigger) string {
	return naming.TruncatedPubsubResourceName("cre-tgr", t.Namespace, t.Name, t.UID)
}

// GenerateRetrySubscriptionName generates a deterministic subscription name
// for a Trigger. If the subscription name would be longer than allowed by
// PubSub, the Trigger name is truncated to fit.
func GenerateRetrySubscriptionName(t *brokerv1beta1.Trigger) string {
	return naming.TruncatedPubsubResourceName("cre-tgr", t.Namespace, t.Name, t.UID)
}
