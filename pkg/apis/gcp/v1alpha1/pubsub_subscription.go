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

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubSubscription is the Schema for the gcppubsubsources API.
// +k8s:openapi-gen=true
type PubSubSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PubSubSubscriptionSpec `json:"spec,omitempty"`
	Status PubSubStatus           `json:"status,omitempty"`
}

// Check that PubSubSubscription can be validated and can be defaulted.
var _ runtime.Object = (*PubSubSubscription)(nil)

// Check that PubSubSubscription will be checked for immutable fields.
var _ apis.Immutable = (*PubSubSubscription)(nil)

// Check that PubSubSubscription implements the Conditions duck type.
var _ = duck.VerifyType(&PubSubSubscription{}, &duckv1alpha1.Conditions{})

// PubSubSubscriptionSpec defines the desired state of the PubSubSubscription.
type PubSubSubscriptionSpec struct {
	// GcpCredsSecret is the credential to use to poll the GCP PubSubSubscription Subscription. It is not used
	// to create or delete the Subscription, only to poll it. The value of the secret entry must be
	// a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	GcpCredsSecret corev1.SecretKeySelector `json:"gcpCredsSecret,omitempty"`

	// GoogleCloudProject is the ID of the Google Cloud Project that the PubSubSubscription Topic exists in.
	GoogleCloudProject string `json:"googleCloudProject,omitempty"`

	// Topic is the ID of the GCP PubSubSubscription Topic to Subscribe to. It must be in the form of the
	// unique identifier within the project, not the entire name. E.g. it must be 'laconia', not
	// 'projects/my-gcp-project/topics/laconia'.
	Topic string `json:"topic,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// PubSubEventType is the GcpPubSub CloudEvent type, in case PubSubSubscription doesn't send a
	// CloudEvent itself.
	PubSubEventType = "google.pubsub.topic.publish"
)

// GetPubSub returns the GcpPubSub CloudEvent source value.
func GetPubSub(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", googleCloudProject, topic)
}

const (
	// PubSubSubscriptionConditionReady has status True when the PubSubSubscription is ready to send events.
	PubSubSubscriptionConditionReady = duckv1alpha1.ConditionReady

	// PubSubSubscriptionConditionSinkProvided has status True when the PubSubSubscription has been configured with a sink target.
	PubSubSubscriptionConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// PubSubSubscriptionConditionDeployed has status True when the PubSubSubscription has had it's receive adapter deployment created.
	PubSubSubscriptionConditionDeployed duckv1alpha1.ConditionType = "Deployed"

	// PubSubSubscriptionConditionSubscribed has status True when a GCP PubSubSubscription Subscription has been created pointing at the created receive adapter deployment.
	PubSubSubscriptionConditionSubscribed duckv1alpha1.ConditionType = "Subscribed"

	// PubSubSubscriptionConditionEventTypesProvided has status True when the PubSubSubscription has been configured with event types.
	PubSubSubscriptionConditionEventTypesProvided duckv1alpha1.ConditionType = "EventTypesProvided"
)

// PubSubStatus defines the observed state of PubSubSubscription.
type PubSubStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the PubSubSubscription.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubSubscriptionList contains a list of PubSubs.
type PubSubSubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PubSubSubscription `json:"items"`
}
