/*
Copyright 2020 Google LLC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"
)

const (
	// DependencyAnnotation is the annotation key used to mark the sources that the Trigger depends on.
	// This will be used when the kn client creates a source and trigger pair for the user such that the trigger only receives events produced by the paired source.
	DependencyAnnotation = "knative.dev/dependency"
	// InjectionAnnotation is the annotation key used to enable knative eventing injection for a namespace and automatically create a default broker.
	// This will be used when the client creates a trigger paired with default broker and the default broker doesn't exist in the namespace
	InjectionAnnotation = "knative-eventing-injection"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trigger represents a request to have events delivered to a consumer from a
// Broker's event pool.
type Trigger struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Trigger.
	Spec eventingv1beta1.TriggerSpec `json:"spec,omitempty"`

	// Status represents the current state of the Trigger. This data may be out of
	// date.
	// +optional
	Status TriggerStatus `json:"status,omitempty"`
}

var (
	// Check that Trigger can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*Trigger)(nil)
	_ apis.Defaultable = (*Trigger)(nil)

	// Check that Trigger can return its spec untyped.
	_ apis.HasSpec = (*Trigger)(nil)

	_ runtime.Object = (*Trigger)(nil)

	// Check that we can create OwnerReferences to a Trigger.
	_ kmeta.OwnerRefable = (*Trigger)(nil)
)

// TriggerStatus represents the current state of a Trigger.
type TriggerStatus struct {
	eventingv1beta1.TriggerStatus `json:",inline"`

	//TODO these fields don't work yet.
	//TODO this requires updating the eventing webhook to allow unknown fields. Since the only unknown
	// fields required are in status, maybe we can use a separate webhook just for broker and trigger
	// status with allow unknown fields set.

	// ProjectID is the resolved project ID in use by the Trigger's PubSub resources.
	// +optional
	//ProjectID string `json:"projectId,omitempty"`

	// TopicID is the created topic ID used by the Broker.
	// +optional
	//TopicID string `json:"topicId,omitempty"`

	// SubscriptionID is the created subscription ID used by the Broker.
	// +optional
	//SubscriptionID string `json:"subscriptionId,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TriggerList is a collection of Triggers.
type TriggerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trigger `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Triggers.
func (t *Trigger) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Trigger")
}

// GetUntypedSpec returns the spec of the Trigger.
func (t *Trigger) GetUntypedSpec() interface{} {
	return t.Spec
}
