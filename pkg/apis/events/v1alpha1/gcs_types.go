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
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCS is a specification for a GCS Source resource
type GCS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCSSpec   `json:"spec"`
	Status GCSStatus `json:"status"`
}

// Check that GCS implements the Conditions duck type.
var _ = duck.VerifyType(&GCS{}, &duckv1alpha1.Conditions{})

// GCSSpec is the spec for a GCS resource
type GCSSpec struct {
	// GCSCredsSecret is the credential to use to create the Notification on the GCS bucket.
	// The value of the secret entry must be a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	GCSCredsSecret corev1.SecretKeySelector `json:"gcsCredsSecret"`

	// GcpCredsSecret is the credential to use to poll the GCP PubSub Subscription. It is not used
	// to create or delete the Subscription, only to poll it. The value of the secret entry must be
	// a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	// If omitted, uses GCSCredsSecret from above
	// +optional
	GcpCredsSecret *corev1.SecretKeySelector `json:"gcpCredsSecret,omitempty"`

	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GCS exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// GoogleCloudProject is the ID of the Google Cloud Project that the PubSub Topic exists in.
	GoogleCloudProject string `json:"googleCloudProject,omitempty"`

	// Bucket to subscribe to
	Bucket string `json:"bucket"`

	// EventTypes to subscribe to
	EventTypes []string `json:"eventTypes,omitempty"`

	// ObjectNamePrefix limits the notifications to objects with this prefix
	// +optional
	ObjectNamePrefix string `json:"objectNamePrefix,omitempty"`

	// CustomAttributes is the optional list of additional attributes to attach to each Cloud PubSub
	// message published for this notification subscription.
	// +optional
	CustomAttributes map[string]string `json:"customAttributes,omitempty"`

	// PayloadFormat specifies the contents of the message payload.
	// See https://cloud.google.com/storage/docs/pubsub-notifications#payload.
	// +optional
	PayloadFormat string `json:"payloadFormat,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use
	// as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

const (
	// GCSConditionReady has status True when the GCS is ready to send events.
	GCSConditionReady = duckv1alpha1.ConditionReady

	// PubSubSourceReady has status True when the underlying GCP PubSub Source is ready
	PubSubSourceReady duckv1alpha1.ConditionType = "PubSubSourceReady"

	// PubSubTopicReady has status True when the underlying GCP PubSub topic is ready
	PubSubTopicReady duckv1alpha1.ConditionType = "PubSubTopicReady"

	// GCSReady has status True when GCS has been configured properly to send Notification events
	GCSReady duckv1alpha1.ConditionType = "GCSReady"
)

var gcsSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	PubSubSourceReady,
	PubSubTopicReady,
	GCSReady)

// GCSStatus is the status for a GCS resource
type GCSStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

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

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GCSStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return gcsSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *GCSStatus) IsReady() bool {
	return gcsSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *GCSStatus) InitializeConditions() {
	gcsSourceCondSet.Manage(s).InitializeConditions()
}

// MarkPubSubNotSourceReady sets the condition that the underlying PubSub source is not ready and why
func (s *GCSStatus) MarkPubSubSourceNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(PubSubSourceReady, reason, messageFormat, messageA...)
}

// MarkPubSubSourceReady sets the condition that the underlying PubSub source is ready
func (s *GCSStatus) MarkPubSubSourceReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(PubSubSourceReady)
}

// MarkPubSubTopicNotReady sets the condition that the PubSub topic was not created and why
func (s *GCSStatus) MarkPubSubTopicNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(PubSubTopicReady, reason, messageFormat, messageA...)
}

// MarkPubSubTopicReady sets the condition that the underlying PubSub topic was created successfully
func (s *GCSStatus) MarkPubSubTopicReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(PubSubTopicReady)
}

// MarkGCSNotReady sets the condition that the GCS has been configured to send Notifications
func (s *GCSStatus) MarkGCSNotReady(reason, messageFormat string, messageA ...interface{}) {
	gcsSourceCondSet.Manage(s).MarkFalse(GCSReady, reason, messageFormat, messageA...)
}

func (s *GCSStatus) MarkGCSReady() {
	gcsSourceCondSet.Manage(s).MarkTrue(GCSReady)
}

func (gcsSource *GCS) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GCS")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCSList is a list of GCS resources
type GCSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GCS `json:"items"`
}
