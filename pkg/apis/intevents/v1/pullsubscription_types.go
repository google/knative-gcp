/*
Copyright 2020 Google LLC.

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

package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/knative-gcp/pkg/apis/duck/v1"
	"github.com/google/knative-gcp/pkg/apis/intevents"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PullSubscription is the Schema for the gcppullSubscriptions API.
// +k8s:openapi-gen=true
type PullSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullSubscriptionSpec   `json:"spec,omitempty"`
	Status PullSubscriptionStatus `json:"status,omitempty"`
}

// Check that PullSubscription can be converted to other versions.
var _ apis.Convertible = (*PullSubscription)(nil)

// Check that PullSubscription can be validated and can be defaulted.
var _ runtime.Object = (*PullSubscription)(nil)

// Check that PullSubscription implements the Conditions duck type.
var _ = duck.VerifyType(&PullSubscription{}, &duckv1.Conditions{})

// Check that PullSubscription implements the KRShaped duck type.
var _ duckv1.KRShaped = (*PullSubscription)(nil)

// PullSubscriptionSpec defines the desired state of the PullSubscription.
type PullSubscriptionSpec struct {
	v1.PubSubSpec `json:",inline"`

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

	// Transformer is a reference to an object that will resolve to a domain
	// name or a URI directly to use as the transformer or a URI directly.
	// +optional
	Transformer *duckv1.Destination `json:"transformer,omitempty"`

	// AdapterType determines the type of receive adapter that a
	// PullSubscription uses.
	// +optional
	AdapterType string `json:"adapterType,omitempty"`
}

// GetAckDeadline parses AckDeadline and returns the default if an error occurs.
func (ps PullSubscriptionSpec) GetAckDeadline() time.Duration {
	if ps.AckDeadline != nil {
		if duration, err := time.ParseDuration(*ps.AckDeadline); err == nil {
			return duration
		}
	}
	return intevents.DefaultAckDeadline
}

// GetRetentionDuration parses RetentionDuration and returns the default if an error occurs.
func (ps PullSubscriptionSpec) GetRetentionDuration() time.Duration {
	if ps.RetentionDuration != nil {
		if duration, err := time.ParseDuration(*ps.RetentionDuration); err == nil {
			return duration
		}
	}
	return intevents.DefaultRetentionDuration
}

const (
	// PullSubscriptionConditionReady has status True when the PullSubscription is
	// ready to send events.
	PullSubscriptionConditionReady = apis.ConditionReady

	// PullSubscriptionConditionSinkProvided has status True when the PullSubscription
	// has been configured with a sink target.
	PullSubscriptionConditionSinkProvided apis.ConditionType = "SinkProvided"

	// PullSubscriptionConditionDeployed has status True when the PullSubscription has
	// had its data plane resource(s) created.
	PullSubscriptionConditionDeployed apis.ConditionType = "Deployed"

	// PullSubscriptionConditionSubscribed has status True when a Google Cloud
	// Pub/Sub Subscription has been created pointing at the created receive
	// adapter deployment.
	PullSubscriptionConditionSubscribed apis.ConditionType = "Subscribed"

	// PullSubscriptionConditionTransformerProvided has status True when the
	// PullSubscription has been configured with a transformer target.
	PullSubscriptionConditionTransformerProvided apis.ConditionType = "TransformerProvided"
)

var pullSubscriptionCondSet = apis.NewLivingConditionSet(
	PullSubscriptionConditionSinkProvided,
	PullSubscriptionConditionDeployed,
	PullSubscriptionConditionSubscribed,
)

// PullSubscriptionStatus defines the observed state of PullSubscription.
type PullSubscriptionStatus struct {
	v1.PubSubStatus `json:",inline"`

	// TransformerURI is the current active transformer URI that has been
	// configured for the PullSubscription.
	// +optional
	TransformerURI *apis.URL `json:"transformerUri,omitempty"`

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

// GetGroupVersionKind returns the GroupVersion.
func (s *PullSubscription) GetGroupVersion() schema.GroupVersion {
	return SchemeGroupVersion
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *PullSubscription) IdentitySpec() *v1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *PullSubscription) IdentityStatus() *v1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (*PullSubscription) ConditionSet() *apis.ConditionSet {
	return &pullSubscriptionCondSet
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*PullSubscription) GetConditionSet() apis.ConditionSet {
	return pullSubscriptionCondSet
}

// GetStatus retrieves the status of the PullSubscription. Implements the KRShaped interface.
func (s *PullSubscription) GetStatus() *duckv1.Status {
	return &s.Status.Status
}
