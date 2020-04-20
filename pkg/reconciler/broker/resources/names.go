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
	"fmt"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
)

const (
	pubsubMax       = 255
	k8sNamespaceMax = 63
	k8sNameMax      = 253
	uidLength       = 36
	md5Len          = 32
)

//TODO check length < 256 chars and truncate
// pubsub resource max length: 255 chars
// Namespace max length: 63 chars
// broker name max length: 253 chars
// trigger name max length: 253 chars
// uid length: 36 chars
// prefix + separators: 10 chars
// 255 - 10 - 36 - 63 = 146 chars for name

// GenerateDecouplingTopicName generates a deterministic topic name for a
// Broker.
func GenerateDecouplingTopicName(b *brokerv1beta1.Broker) string {
	return fmt.Sprintf("cre-bkr_%s_%s_%s", b.Namespace, b.Name, string(b.UID))
}

// GenerateDecouplingSubscriptionName generates a deterministic subscription
// name for a Broker.
func GenerateDecouplingSubscriptionName(b *brokerv1beta1.Broker) string {
	return fmt.Sprintf("cre-bkr_%s_%s_%s", b.Namespace, b.Name, string(b.UID))
}

// GenerateRetryTopicName generates a deterministic topic name for a Trigger.
func GenerateRetryTopicName(t *brokerv1beta1.Trigger) string {
	return fmt.Sprintf("cre-tgr_%s_%s_%s", t.Namespace, t.Name, string(t.UID))
}

// GenerateRetrySubscriptionName generates a deterministic subscription name
// for a Trigger.
func GenerateRetrySubscriptionName(t *brokerv1beta1.Trigger) string {
	return fmt.Sprintf("cre-tgr_%s_%s_%s", t.Namespace, t.Name, string(t.UID))
}
