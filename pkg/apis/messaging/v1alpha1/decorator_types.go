/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Decorator represents an addressable resource that augments an event passed
// through the decorator to the sink.
type Decorator struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Decorator.
	Spec DecoratorSpec `json:"spec,omitempty"`

	// Status represents the current state of the Decorator. This data may be out of
	// date.
	// +optional
	Status DecoratorStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Decorator)(nil)
var _ apis.Defaultable = (*Decorator)(nil)
var _ runtime.Object = (*Decorator)(nil)
var _ resourcesemantics.GenericCRD = (*Decorator)(nil)

// DecoratorSpec
type DecoratorSpec struct {
	// This brings in CloudEventOverrides and Sink.
	duckv1.SourceSpec `json:",inline"`
}

var decoratorCondSet = apis.NewLivingConditionSet(
	DecoratorConditionAddressable,
	DecoratorConditionSinkProvided,
	DecoratorConditionServiceReady,
)

const (
	// DecoratorConditionReady has status True when all sub-conditions below have
	// been set to True.
	DecoratorConditionReady = apis.ConditionReady

	// DecoratorConditionAddressable has status true when this Decorator meets the
	// Addressable contract and has a non-empty url.
	DecoratorConditionAddressable apis.ConditionType = "Addressable"

	// DecoratorConditionServiceReady has status True when the Decorator has had a
	// Pub/Sub topic created for it.
	DecoratorConditionServiceReady apis.ConditionType = "ServiceReady"

	// DecoratorConditionSinkProvided has status True when the Decorator
	// has been configured with a sink target.
	DecoratorConditionSinkProvided apis.ConditionType = "SinkProvided"
)

// DecoratorStatus represents the current state of a Decorator.
type DecoratorStatus struct {
	// inherits duck/v1beta1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// Channel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1.AddressStatus `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the
	// Decorator.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DecoratorlList is a collection of Decorators.
type DecoratorList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Decorator `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Decorators.
func (d *Decorator) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Decorator")
}
