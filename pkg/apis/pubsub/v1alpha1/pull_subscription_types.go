/*
Copyright 2019 Google LLC

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

package v1alpha1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/apis/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PullSubscription is the Schema for the gcppullSubscriptions API.
// +k8s:openapi-gen=true
type PullSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullSubscriptionSpec   `json:"spec,omitempty"`
	Status PullSubscriptionStatus `json:"status,omitempty"`
}

// PubSubMode returns the mode currently set for PullSubscription.
func (p *PullSubscription) PubSubMode() ModeType {
	return p.Spec.Mode
}

// Check that PullSubscription can be validated and can be defaulted.
var _ runtime.Object = (*PullSubscription)(nil)

// Check that PullSubscription will be checked for immutable fields.
var _ apis.Immutable = (*PullSubscription)(nil)

// Check that PullSubscription implements the Conditions duck type.
var _ = duck.VerifyType(&PullSubscription{}, &duckv1.Conditions{})

// PullSubscriptionSpec defines the desired state of the PullSubscription.
type PullSubscriptionSpec struct {
	// Secret is the credential to use to create and poll the PullSubscription
	// Subscription. The value of the secret entry must be a service account
	// key in the JSON format (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the PullSubscription
	// Topic exists in.
	// +optional
	Project string `json:"project,omitempty"`

	// Topic is the ID of the PullSubscription Topic to Subscribe to. It must
	// be in the form of the unique identifier within the project, not the
	// entire name. E.g. it must be 'laconia', not
	// 'projects/my-proj/topics/laconia'.
	Topic string `json:"topic,omitempty"`

	// AckDeadline is the default maximum time after a subscriber receives a
	// message before the subscriber should acknowledge the message. Defaults
	// to 30 seconds ('30s').
	// +optional
	AckDeadline *string `json:"ackDeadline,omitempty"`

	// RetainAckedMessages defines whether to retain acknowledged messages. If
	// true, acknowledged messages will not be expunged until they fall out of
	// the RetentionDuration window.
	RetainAckedMessages bool `json:"retainAckedMessages,omitempty"`

	// RetentionDuration defines how long to retain messages in backlog, from
	// the time of publish. If RetainAckedMessages is true, this duration
	// affects the retention of acknowledged messages, otherwise only
	// unacknowledged messages are retained. Cannot be longer than 7 days or
	// shorter than 10 minutes. Defaults to 7 days ('7d').
	// +optional
	RetentionDuration *string `json:"retentionDuration,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name or a
	// URI directly to use as the sink.
	Sink v1alpha1.Destination `json:"sink"`

	// Transformer is a reference to an object that will resolve to a domain
	// name or a URI directly to use as the transformer or a URI directly.
	// +optional
	Transformer *v1alpha1.Destination `json:"transformer,omitempty"`

	// Mode defines the encoding and structure of the payload of when the
	// PullSubscription invokes the sink.
	// +optional
	Mode ModeType `json:"mode,omitempty"`

	// CloudEventOverrides defines overrides to control modifications of the
	// event sent to the sink.
	// +optional
	CloudEventOverrides *CloudEventOverrides `json:"ceOverrides,omitempty"`
}

// CloudEventOverrides defines arguments for a Source that control the output
// format of the CloudEvents produced by the Source.
type CloudEventOverrides struct {
	// Extensions specify what attribute are added or overridden on the
	// outbound event. Each `Extensions` key-value pair are set on the event as
	// an attribute extension independently.
	// +optional
	Extensions map[string]string `json:"extensions,omitempty"`
}

// GetAckDeadline parses AckDeadline and returns the default if an error occurs.
func (ps PullSubscriptionSpec) GetAckDeadline() time.Duration {
	if ps.AckDeadline != nil {
		if duration, err := time.ParseDuration(*ps.AckDeadline); err == nil {
			return duration
		}
	}
	return defaultAckDeadline
}

// GetRetentionDuration parses RetentionDuration and returns the default if an error occurs.
func (ps PullSubscriptionSpec) GetRetentionDuration() time.Duration {
	if ps.RetentionDuration != nil {
		if duration, err := time.ParseDuration(*ps.RetentionDuration); err == nil {
			return duration
		}
	}
	return defaultRetentionDuration
}

type ModeType string

const (
	// ModeCloudEventsBinary will use CloudEvents binary HTTP mode with
	// flattened Pub/Sub payload.
	ModeCloudEventsBinary ModeType = "CloudEventsBinary"

	// ModeCloudEventsStructured will use CloudEvents structured HTTP mode with
	// flattened Pub/Sub payload.
	ModeCloudEventsStructured ModeType = "CloudEventsStructured"

	// ModePushCompatible will use CloudEvents binary HTTP mode with expanded
	// Pub/Sub payload that matches how Cloud Pub/Sub delivers a push message.
	ModePushCompatible ModeType = "PushCompatible"
)

// PubSubEventSource returns the Cloud Pub/Sub CloudEvent source value.
func PubSubEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/projects/%s/topics/%s", googleCloudProject, topic)
}

const (
	// PullSubscription CloudEvent type
	PubSubPublish = "com.google.cloud.pubsub.topic.publish"
)

const (
	// PullSubscriptionConditionReady has status True when the PullSubscription is
	// ready to send events.
	PullSubscriptionConditionReady = apis.ConditionReady

	// PullSubscriptionConditionSinkProvided has status True when the PullSubscription
	// has been configured with a sink target.
	PullSubscriptionConditionSinkProvided apis.ConditionType = "SinkProvided"

	// PullSubscriptionConditionDeployed has status True when the PullSubscription has
	// had its receive adapter deployment created.
	PullSubscriptionConditionDeployed apis.ConditionType = "Deployed"

	// PullSubscriptionConditionSubscribed has status True when a Google Cloud
	// Pub/Sub Subscription has been created pointing at the created receive
	// adapter deployment.
	PullSubscriptionConditionSubscribed apis.ConditionType = "Subscribed"

	// PullSubscriptionConditionTransformerProvided has status True when the
	// PullSubscription has been configured with a transformer target.
	PullSubscriptionConditionTransformerProvided apis.ConditionType = "TransformerProvided"

	// PullSubscriptionConditionEventTypesProvided has status True when the
	// PullSubscription has been configured with event types.
	PullSubscriptionConditionEventTypesProvided apis.ConditionType = "EventTypesProvided"
)

var pullSubscriptionCondSet = apis.NewLivingConditionSet(
	PullSubscriptionConditionSinkProvided,
	PullSubscriptionConditionDeployed,
	PullSubscriptionConditionSubscribed,
)

// PullSubscriptionStatus defines the observed state of PullSubscription.
type PullSubscriptionStatus struct {
	// inherits duck/v1beta1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the
	// PullSubscription.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`

	// TransformerURI is the current active transformer URI that has been
	// configured for the PullSubscription.
	// +optional
	TransformerURI string `json:"transformerUri,omitempty"`

	// ProjectID is the resolved project ID in use by the PullSubscription.
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// SubscriptionID is the created subscription ID used by the PullSubscription.
	// +optional
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PullSubscriptionList contains a list of PubSubs.
type PullSubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PullSubscription `json:"items"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *PullSubscription) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("PullSubscription")
}
