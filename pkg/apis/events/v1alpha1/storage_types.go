/*
Copyright 2019 Google LLC.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Storage is a specification for a Google Cloud Storage Source resource
type Storage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageSpec   `json:"spec"`
	Status StorageStatus `json:"status"`
}

// Check that Storage implements the Conditions duck type.
var _ = duck.VerifyType(&Storage{}, &duckv1beta1.Conditions{})

// StorageSpec is the spec for a Storage resource
type StorageSpec struct {
	// GCSCredsSecret is the credential to use to create the Notification on the GCS bucket.
	// The value of the secret entry must be a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	GCSSecret corev1.SecretKeySelector `json:"gcsSecret"`

	// PullSubsciptionSecret is the credential to use for the GCP PubSub Subscription.
	// It is used for the PullSubscription that is used to deliver events from GCS.
	// The value of the secret entry must be a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	// If omitted, uses GCSSecret from above
	// +optional
	PullSubscriptionSecret *corev1.SecretKeySelector `json:"pullSubscriptionSecret,omitempty"`

	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GCS exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Project is the ID of the Google Cloud Project that the Bucket being
	// subscribed to exists in.
	Project string `json:"Project,omitempty"`

	// Bucket to subscribe to.
	Bucket string `json:"bucket"`

	// EventTypes to subscribe to.
	EventTypes []string `json:"eventTypes,omitempty"`

	// ObjectNamePrefix limits the notifications to objects with this prefix
	// +optional
	ObjectNamePrefix string `json:"objectNamePrefix,omitempty"`

	// CustomAttributes is the optional list of additional attributes to attach to each Cloud PubSub
	// message published for this notification subscription.
	// +optional
	CloudEventOverrides *duckv1beta1.CloudEventOverrides `json:"ceOverrides,omitempty"`

	// PayloadFormat specifies the contents of the message payload.
	// See https://cloud.google.com/storage/docs/pubsub-notifications#payload.
	// +optional
	PayloadFormat string `json:"payloadFormat,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use
	// as the sink.
	Sink corev1.ObjectReference `json:"sink"`
}

const (
	// StorageConditionReady has status True when the Storage is ready to send events.
	StorageConditionReady = apis.ConditionReady

	// PullSubscriptionReady has status True when the underlying PullSubscription is ready
	PullSubscriptionReady apis.ConditionType = "PullSubscriptionReady"

	// PubSubTopicReady has status True when the underlying GCP PubSub topic is ready
	PubSubTopicReady apis.ConditionType = "PubSubTopicReady"

	// GCSReady has status True when GCS has been configured properly to send Notification events
	GCSReady apis.ConditionType = "GCSReady"
)

var gcsSourceCondSet = apis.NewLivingConditionSet(
	PullSubscriptionReady,
	PubSubTopicReady,
	GCSReady)

// StorageStatus is the status for a GCS resource
type StorageStatus struct {
	// inherits duck/v1beta1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// TODO: add conditions and other stuff here...
	// NotificationID is the ID that GCS identifies this notification as.
	// +optional
	NotificationID string `json:"notificationID,omitempty"`

	// Topic where the notifications are sent to.
	// +optional
	Topic string `json:"topic,omitempty"`

	// SinkURI is the current active sink URI that has been configured for the GCS.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

func (storage *Storage) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Storage")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageList is a list of Storage resources
type StorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Storage `json:"items"`
}
