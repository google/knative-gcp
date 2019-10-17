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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSub is the Schema for the gcp pubsub API.
// +k8s:openapi-gen=true
type PubSub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PubSubSpec   `json:"spec,omitempty"`
	Status PubSubStatus `json:"status,omitempty"`
}

// Check that PubSub can be validated and can be defaulted.
var _ runtime.Object = (*PubSub)(nil)

// Check that PubSub will be checked for immutable fields.
var _ apis.Immutable = (*PubSub)(nil)

// Check that PubSub implements the Conditions duck type.
var _ = duck.VerifyType(&PubSub{}, &duckv1.Conditions{})

// PubSubSpec defines the desired state of the PubSub.
type PubSubSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1alpha1.PubSubSpec

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
func (ps PubSubSpec) GetAckDeadline() time.Duration {
	if ps.AckDeadline != nil {
		if duration, err := time.ParseDuration(*ps.AckDeadline); err == nil {
			return duration
		}
	}
	return defaultAckDeadline
}

// GetRetentionDuration parses RetentionDuration and returns the default if an error occurs.
func (ps PubSubSpec) GetRetentionDuration() time.Duration {
	if ps.RetentionDuration != nil {
		if duration, err := time.ParseDuration(*ps.RetentionDuration); err == nil {
			return duration
		}
	}
	return defaultRetentionDuration
}

// PubSubEventSource returns the Cloud Pub/Sub CloudEvent source value.
func PubSubEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/projects/%s/topics/%s", googleCloudProject, topic)
}

const (
	// PubSub CloudEvent type
	PubSubPublish = "com.google.cloud.pubsub.topic.publish"
)

const (
	// PubSubConditionReady has status True when the PubSub is
	// ready to send events.
	PubSubConditionReady = apis.ConditionReady
)

var PubSubCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
)

// PubSubStatus defines the observed state of PubSub.
type PubSubStatus struct {
	// This brings in duck/v1beta1 Status as well as SinkURI
	duckv1.SourceStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubList contains a list of PubSubs.
type PubSubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PubSub `json:"items"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *PubSub) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("PubSub")
}

// Methods for pubsubable interface

// PubSubSpec returns the PubSubSpec portion of the Spec.
func (ps *PubSub) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &ps.Spec.PubSubSpec
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (ps *PubSub) ConditionSet() *apis.ConditionSet {
	return &PubSubCondSet
}
