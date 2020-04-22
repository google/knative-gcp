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

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/webhook/resourcesemantics"

	"github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trigger is a specification for a Trigger resource
type Trigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TriggerSpec   `json:"spec"`
	Status TriggerStatus `json:"status"`
}

// Check that Trigger can be validated, can be defaulted, and has immutable fields.
var _ runtime.Object = (*Trigger)(nil)
var _ resourcesemantics.GenericCRD = (*Trigger)(nil)

// Check that Trigger implements the Conditions duck type.
var _ = duck.VerifyType(&Trigger{}, &duckv1.Conditions{})

// TriggerSpec is the spec for a Trigger resource
type TriggerSpec struct {
	v1beta1.IdentitySpec `json:",inline"`

	// Secret is the credential to be used to create the trigger
	// in EventFlow. The value of the secret entry must be a service
	// account key in the JSON format
	// (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the trigger
	// will be created in.
	Project string `json:"project,omitempty"`

	// Type of Trigger, only AUDIT is supported
	SourceType string `json:"sourceType"`

	// String to string Attributes (key, value) to filter events by exact
	// match on event context attributes. For each `Filters` key-value pair,
	// the key in the map is compared with the equivalent key in the event
	// context. An event passes the filter if all values are equal to the
	// specified values.
	Filters map[string]string `json:"filters"`

	// Trigger is the ID of the Trigger to create/use in Event Flow.
	Trigger string `json:"trigger,omitempty"`
}

const (
	// CloudGoogleEvent types used by Trigger.
	TriggerAuditLogs = "AUDIT"

	// CloudGoogleEvent source prefix.
	// TODO(nlopezgi): fix this once prefix in eventflow is defined
	triggerPrefix = "//triggers.googleapis.com/projects" //
)
const (
	// TriggerConditionReady has status True when the Trigger is ready to send events.
	TriggerConditionReady = apis.ConditionReady

	// TriggerReady has status True when <EventFlow> has been configured properly to
	// send Trigger events
	TriggerReady apis.ConditionType = "TriggerReady"
)

func TriggerEventSource(googleCloudProject, sourceType string) string {
	// TODO(nlopezgi): fix this to include something about filter?
	return fmt.Sprintf("%s/%s/trigger/%s", triggerPrefix, googleCloudProject, sourceType)
}

var triggerCondSet = apis.NewLivingConditionSet(
	TriggerReady)

// TriggerStatus is the status for a Trigger resource
type TriggerStatus struct {
	v1beta1.IdentityStatus `json:",inline"`

	// ProjectID is the resolved project ID in use by the Topic.
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// TriggerID is the ID that Event Flow identifies this trigger.
	// +optional
	TriggerID string `json:"triggerId,omitempty"`
}

func (s *Trigger) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Trigger")
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *Trigger) IdentitySpec() *v1beta1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *Trigger) IdentityStatus() *v1beta1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *Trigger) ConditionSet() *apis.ConditionSet {
	return &triggerCondSet
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TriggerList is a list of Trigger resources
type TriggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Trigger `json:"items"`
}
