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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook"
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

var (
	_ apis.Validatable   = (*Storage)(nil)
	_ apis.Defaultable   = (*Storage)(nil)
	_ runtime.Object     = (*Storage)(nil)
	_ kmeta.OwnerRefable = (*Storage)(nil)
	_ webhook.GenericCRD = (*Storage)(nil)
)

// StorageSpec is the spec for a Storage resource
type StorageSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1alpha1.PubSubSpec

	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GCS exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

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
	// CloudEvent types used by Storage.
	StorageFinalize       = "google.storage.object.finalize"
	StorageArchive        = "google.storage.object.archive"
	StorageDelete         = "google.storage.object.delete"
	StorageMetadataUpdate = "google.storage.object.metadataUpdate"

	// CloudEvent source prefix.
	storageSourcePrefix = "//storage.googleapis.com/buckets"
)

const (
	// StorageConditionReady has status True when the Storage is ready to send events.
	StorageConditionReady = apis.ConditionReady

	// NotificationReady has status True when GCS has been configured properly to
	// send Notification events
	NotificationReady apis.ConditionType = "NotificationReady"
)

func StorageEventSource(bucket string) string {
	return fmt.Sprintf("%s/%s", storageSourcePrefix, bucket)
}

var StorageCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
	duckv1alpha1.TopicReady,
	NotificationReady)

// StorageStatus is the status for a GCS resource
type StorageStatus struct {
	// This brings in our GCP PubSub based events importers
	// duck/v1beta1 Status, SinkURI, ProjectID, TopicID, and SubscriptionID
	duckv1alpha1.PubSubStatus

	// NotificationID is the ID that GCS identifies this notification as.
	// +optional
	NotificationID string `json:"notificationId,omitempty"`
}

func (storage *Storage) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Storage")
}

// Methods for pubsubable interface

// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *Storage) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *Storage) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *Storage) ConditionSet() *apis.ConditionSet {
	return &StorageCondSet
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageList is a list of Storage resources
type StorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Storage `json:"items"`
}
