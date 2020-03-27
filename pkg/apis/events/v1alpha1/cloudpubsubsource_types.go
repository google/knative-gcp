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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	kngcpduck "github.com/google/knative-gcp/pkg/duck"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudPubSubSource is a specification for a CloudPubSubSource resource
// +k8s:openapi-gen=true
type CloudPubSubSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudPubSubSourceSpec   `json:"spec,omitempty"`
	Status CloudPubSubSourceStatus `json:"status,omitempty"`
}

var (
	_ kmeta.OwnerRefable           = (*CloudPubSubSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudPubSubSource)(nil)
	_ kngcpduck.PubSubable         = (*CloudPubSubSource)(nil)
	_ kngcpduck.Identifiable       = (*CloudPubSubSource)(nil)
	_                              = duck.VerifyType(&CloudPubSubSource{}, &duckv1.Conditions{})
)

// CloudPubSubSourceSpec defines the desired state of the CloudPubSubSource.
type CloudPubSubSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1alpha1.PubSubSpec `json:",inline"`

	// Topic is the ID of the PubSub Topic to Subscribe to. It must
	// be in the form of the unique identifier within the project, not the
	// entire name. E.g. it must be 'laconia', not
	// 'projects/my-proj/topics/laconia'.
	Topic string `json:"topic"`

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
}

// GetAckDeadline parses AckDeadline and returns the default if an error occurs.
func (ps CloudPubSubSourceSpec) GetAckDeadline() time.Duration {
	if ps.AckDeadline != nil {
		if duration, err := time.ParseDuration(*ps.AckDeadline); err == nil {
			return duration
		}
	}
	return defaultAckDeadline
}

// GetRetentionDuration parses RetentionDuration and returns the default if an error occurs.
func (ps CloudPubSubSourceSpec) GetRetentionDuration() time.Duration {
	if ps.RetentionDuration != nil {
		if duration, err := time.ParseDuration(*ps.RetentionDuration); err == nil {
			return duration
		}
	}
	return defaultRetentionDuration
}

// CloudPubSubSourceEventSource returns the Cloud Pub/Sub CloudEvent source value.
func CloudPubSubSourceEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/projects/%s/topics/%s", googleCloudProject, topic)
}

const (
	// CloudPubSubSource CloudEvent type
	CloudPubSubSourcePublish = "com.google.cloud.pubsub.topic.publish"
)

const (
	// CloudPubSubSourceConditionReady has status True when the CloudPubSubSource is
	// ready to send events.
	CloudPubSubSourceConditionReady = apis.ConditionReady
)

var pubSubCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
)

// CloudPubSubSourceStatus defines the observed state of CloudPubSubSource.
type CloudPubSubSourceStatus struct {
	// This brings in duck/v1beta1 Status as well as SinkURI
	duckv1alpha1.PubSubStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudPubSubSourceList contains a list of CloudPubSubSources.
type CloudPubSubSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudPubSubSource `json:"items"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *CloudPubSubSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CloudPubSubSource")
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *CloudPubSubSource) IdentitySpec() *duckv1alpha1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudPubSubSource) IdentityStatus() *duckv1alpha1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (ps *CloudPubSubSource) ConditionSet() *apis.ConditionSet {
	return &pubSubCondSet
}

// Methods for pubsubable interface.

// CloudPubSubSourceSpec returns the CloudPubSubSourceSpec portion of the Spec.
func (ps *CloudPubSubSource) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &ps.Spec.PubSubSpec
}

func (s *CloudPubSubSource) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &s.Status.PubSubStatus
}
