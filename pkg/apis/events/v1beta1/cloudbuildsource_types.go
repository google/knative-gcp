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
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	duckv1beta1 "github.com/google/knative-gcp/pkg/apis/duck/v1beta1"
	kngcpduckv1 "github.com/google/knative-gcp/pkg/duck/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// CloudBuildSource is a specification for a CloudBuildSource resource
// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CloudBuildSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudBuildSourceSpec   `json:"spec,omitempty"`
	Status CloudBuildSourceStatus `json:"status,omitempty"`
}

var (
	_ kmeta.OwnerRefable           = (*CloudBuildSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudBuildSource)(nil)
	_ kngcpduckv1.PubSubable       = (*CloudBuildSource)(nil)
	_ kngcpduckv1.Identifiable     = (*CloudBuildSource)(nil)
	_                              = duck.VerifyType(&CloudBuildSource{}, &duckv1.Conditions{})
	_ duckv1.KRShaped              = (*CloudBuildSource)(nil)
)

// CloudBuildSourceSpec defines the desired state of the CloudBuildSource.
type CloudBuildSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret and Project
	gcpduckv1.PubSubSpec `json:",inline"`
}

const (
	// CloudBuildSourceConditionReady has status True when the CloudBuildSource is
	// ready to send events.
	CloudBuildSourceConditionReady = apis.ConditionReady
)

var buildCondSet = apis.NewLivingConditionSet(
	duckv1beta1.PullSubscriptionReady,
)

// CloudBuildSourceStatus defines the observed state of CloudBuildSource.
type CloudBuildSourceStatus struct {
	gcpduckv1.PubSubStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudBuildSourceList contains a list of CloudBuildSources.
type CloudBuildSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudBuildSource `json:"items"`
}

// Methods for pubsubable interface
func (*CloudBuildSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CloudBuildSource")
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *CloudBuildSource) IdentitySpec() *gcpduckv1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudBuildSource) IdentityStatus() *gcpduckv1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// CloudBuildSourceSpec returns the CloudBuildSourceSpec portion of the Spec.
func (bs *CloudBuildSource) PubSubSpec() *gcpduckv1.PubSubSpec {
	return &bs.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (bs *CloudBuildSource) PubSubStatus() *gcpduckv1.PubSubStatus {
	return &bs.Status.PubSubStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object.
func (bs *CloudBuildSource) ConditionSet() *apis.ConditionSet {
	return &buildCondSet
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*CloudBuildSource) GetConditionSet() apis.ConditionSet {
	return buildCondSet
}

// GetStatus retrieves the status of the CloudBuildSource. Implements the KRShaped interface.
func (s *CloudBuildSource) GetStatus() *duckv1.Status {
	return &s.Status.Status
}
