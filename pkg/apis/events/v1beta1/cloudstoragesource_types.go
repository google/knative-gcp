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

package v1beta1

import (
	"fmt"

	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	kngcpduck "github.com/google/knative-gcp/pkg/duck/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudStorageSource is a specification for a Google Cloud CloudStorageSource Source resource
type CloudStorageSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudStorageSourceSpec   `json:"spec"`
	Status CloudStorageSourceStatus `json:"status"`
}

// Verify that CloudStorageSource matches various duck types.
var (
	_ apis.Convertible             = (*CloudStorageSource)(nil)
	_ apis.Defaultable             = (*CloudStorageSource)(nil)
	_ apis.Validatable             = (*CloudStorageSource)(nil)
	_ runtime.Object               = (*CloudStorageSource)(nil)
	_ kmeta.OwnerRefable           = (*CloudStorageSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudStorageSource)(nil)
	_ kngcpduck.Identifiable       = (*CloudStorageSource)(nil)
	_ kngcpduck.PubSubable         = (*CloudStorageSource)(nil)
	_ duckv1.KRShaped              = (*CloudStorageSource)(nil)
)

// CloudStorageSourceSpec is the spec for a CloudStorageSource resource
type CloudStorageSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1beta1.PubSubSpec `json:",inline"`

	// Bucket to subscribe to.
	Bucket string `json:"bucket"`

	// EventTypes to subscribe to. If unspecified, then subscribe to all events.
	// +optional
	EventTypes []string `json:"eventTypes,omitempty"`

	// ObjectNamePrefix limits the notifications to objects with this prefix
	// +optional
	ObjectNamePrefix string `json:"objectNamePrefix,omitempty"`

	// PayloadFormat specifies the contents of the message payload.
	// See https://cloud.google.com/storage/docs/pubsub-notifications#payload.
	// +optional
	PayloadFormat string `json:"payloadFormat,omitempty"`
}

const (
	// CloudEvent types used by CloudStorageSource.
	// TODO: somehow reference the proto values directly.
	CloudStorageSourceObjectFinalizedEventType       = "google.cloud.storage.object.v1.finalized"
	CloudStorageSourceObjectArchivedEventType        = "google.cloud.storage.object.v1.archived"
	CloudStorageSourceObjectDeletedEventType         = "google.cloud.storage.object.v1.deleted"
	CloudStorageSourceObjectMetadataUpdatedEventType = "google.cloud.storage.object.v1.metadataUpdated"
	CloudStorageSourceEventDataSchema                = "https://github.com/googleapis/google-cloudevents/blob/master/proto/google/events/cloud/storage/v1/events.proto"

	// CloudEvent source prefix.
	storageSourcePrefix = "//storage.googleapis.com/buckets"
)

const (
	// CloudStorageSourceConditionReady has status True when the CloudStorageSource is ready to send events.
	CloudStorageSourceConditionReady = apis.ConditionReady

	// NotificationReady has status True when GCS has been configured properly to
	// send Notification events
	NotificationReady apis.ConditionType = "NotificationReady"
)

func CloudStorageSourceEventSource(bucket string) string {
	return fmt.Sprintf("%s/%s", storageSourcePrefix, bucket)
}

var storageCondSet = apis.NewLivingConditionSet(
	duckv1beta1.PullSubscriptionReady,
	duckv1beta1.TopicReady,
	NotificationReady)

// CloudStorageSourceStatus is the status for a GCS resource
type CloudStorageSourceStatus struct {
	// This brings in our GCP PubSub based events importers
	// duck/v1beta1 Status, SinkURI, ProjectID, TopicID, and SubscriptionID
	duckv1beta1.PubSubStatus `json:",inline"`

	// NotificationID is the ID that GCS identifies this notification as.
	// +optional
	NotificationID string `json:"notificationId,omitempty"`
}

func (storage *CloudStorageSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CloudStorageSource")
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *CloudStorageSource) IdentitySpec() *duckv1beta1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudStorageSource) IdentityStatus() *duckv1beta1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *CloudStorageSource) ConditionSet() *apis.ConditionSet {
	return &storageCondSet
}

// Methods for pubsubable interface.

// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *CloudStorageSource) PubSubSpec() *duckv1beta1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *CloudStorageSource) PubSubStatus() *duckv1beta1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudStorageSourceList is a list of CloudStorageSource resources
type CloudStorageSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CloudStorageSource `json:"items"`
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*CloudStorageSource) GetConditionSet() apis.ConditionSet {
	return storageCondSet
}

// GetStatus retrieves the status of the CloudStorageSource. Implements the KRShaped interface.
func (s *CloudStorageSource) GetStatus() *duckv1.Status {
	return &s.Status.Status
}
