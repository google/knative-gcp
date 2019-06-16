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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// Check that PullSubscription can be validated and can be defaulted.
var _ runtime.Object = (*PullSubscription)(nil)

// Check that PullSubscription will be checked for immutable fields.
var _ apis.Immutable = (*PullSubscription)(nil)

// Check that PullSubscription implements the Conditions duck type.
var _ = duck.VerifyType(&PullSubscription{}, &duckv1alpha1.Conditions{})

// PullSubscriptionSpec defines the desired state of the PullSubscription.
type PullSubscriptionSpec struct {
	// Secret is the credential to use to create and poll the PullSubscription
	// Subscription. The value of the secret entry must be a service account
	// key in the JSON format (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the PullSubscription
	// Topic exists in.
	Project string `json:"project,omitempty"`

	// Topic is the ID of the PullSubscription Topic to Subscribe to. It must be in
	// the form of the unique identifier within the project, not the entire
	// name. E.g. it must be 'laconia', not 'projects/my-proj/topics/laconia'.
	Topic string `json:"topic,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to
	// use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// Transformer is a reference to an object that will resolve to a domain
	// name to use as the transformer.
	// +optional
	Transformer *corev1.ObjectReference `json:"transformer,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to
	// run the Receive Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// PubSubEventType is the GcpPubSub CloudEvent type, in case PullSubscription
	// doesn't send a CloudEvent itself.
	PubSubEventType = "google.pubsub.topic.publish"
)

// PubSubEventSource returns the Cloud Pub/Sub CloudEvent source value.
func PubSubEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", googleCloudProject, topic)
}

const (
	// PullSubscriptionConditionReady has status True when the PullSubscription is
	// ready to send events.
	PullSubscriptionConditionReady = duckv1alpha1.ConditionReady

	// PullSubscriptionConditionSinkProvided has status True when the PullSubscription
	// has been configured with a sink target.
	PullSubscriptionConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// PullSubscriptionConditionDeployed has status True when the PullSubscription has
	// had it's receive adapter deployment created.
	PullSubscriptionConditionDeployed duckv1alpha1.ConditionType = "Deployed"

	// PullSubscriptionConditionSubscribed has status True when a Google Cloud
	// Pub/Sub Subscription has been created pointing at the created receive
	// adapter deployment.
	PullSubscriptionConditionSubscribed duckv1alpha1.ConditionType = "Subscribed"

	// PullSubscriptionConditionTransformerProvided has status True when the
	// PullSubscription has been configured with a transformer target.
	PullSubscriptionConditionTransformerProvided duckv1alpha1.ConditionType = "TransformerProvided"

	// PullSubscriptionConditionEventTypesProvided has status True when the
	// PullSubscription has been configured with event types.
	PullSubscriptionConditionEventTypesProvided duckv1alpha1.ConditionType = "EventTypesProvided"
)

var pullSubscriptionCondSet = duckv1alpha1.NewLivingConditionSet(
	PullSubscriptionConditionSinkProvided,
	PullSubscriptionConditionDeployed,
	PullSubscriptionConditionSubscribed,
)

// PullSubscriptionStatus defines the observed state of PullSubscription.
type PullSubscriptionStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's
	//   current state.
	duckv1alpha1.Status `json:",inline"`

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
