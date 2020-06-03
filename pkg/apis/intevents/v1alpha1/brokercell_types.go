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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

const (
	// Annotations to tell if the brokercell is created automatically by the GCP broker controller.
	CreatorKey = "internal.events.cloud.google.com/creator"
	Creator    = "googlecloud"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BrokerCell manages the set of data plane components servicing
// one or more Broker objects and their associated Triggers.
type BrokerCell struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the BrokerCell.
	Spec BrokerCellSpec `json:"spec,omitempty"`

	// Status represents the current state of the BrokerCell. This data may be out of
	// date.
	// +optional
	Status BrokerCellStatus `json:"status,omitempty"`
}

var (
	// Check that BrokerCell can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*BrokerCell)(nil)
	_ apis.Defaultable = (*BrokerCell)(nil)

	// Check that BrokerCell can return its spec untyped.
	_ apis.HasSpec = (*BrokerCell)(nil)

	_ runtime.Object = (*BrokerCell)(nil)

	// Check that we can create OwnerReferences to a BrokerCell.
	_ kmeta.OwnerRefable = (*BrokerCell)(nil)
)

// BrokerCellSpec defines the desired state of a Brokercell.
type BrokerCellSpec struct {
}

// BrokerCellStatus represents the current state of a BrokerCell.
type BrokerCellStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// IngressTemplate contains a URI template as specified by RFC6570 to
	// generate Broker ingress URIs. It may contain variables `name` and
	// `namespace`.
	// Example: "http://broker-ingress.cloud-run-events.svc.cluster.local/{namespace}/{name}"
	IngressTemplate string `json:"ingressTemplate,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BrokerCellList is a collection of BrokerCells.
type BrokerCellList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []BrokerCell `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Brokers
func (bc *BrokerCell) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("BrokerCell")
}

// GetUntypedSpec returns the spec of the BrokerCell.
func (bc *BrokerCell) GetUntypedSpec() interface{} {
	return bc.Spec
}
