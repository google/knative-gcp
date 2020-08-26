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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	kngcpduck "github.com/google/knative-gcp/pkg/duck/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

// CloudBuildSource is a specification for a CloudBuildSource resource.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CloudBuildSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudBuildSourceSpec   `json:"spec,omitempty"`
	Status CloudBuildSourceStatus `json:"status,omitempty"`
}

// Verify that CloudBuildSource matches various duck types.
var (
	_ apis.Convertible             = (*CloudBuildSource)(nil)
	_ apis.Defaultable             = (*CloudBuildSource)(nil)
	_ apis.Validatable             = (*CloudBuildSource)(nil)
	_ runtime.Object               = (*CloudBuildSource)(nil)
	_ kmeta.OwnerRefable           = (*CloudBuildSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudBuildSource)(nil)
	_ kngcpduck.Identifiable       = (*CloudBuildSource)(nil)
	_ kngcpduck.PubSubable         = (*CloudBuildSource)(nil)
)

// CloudBuildSourceSpec defines the desired state of the CloudBuildSource.
type CloudBuildSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret and Project
	duckv1alpha1.PubSubSpec `json:",inline"`

	// Topic is the ID of the PubSub Topic to Subscribe to. It must
	// be in the form of the unique identifier within the project, not the
	// entire name. E.g. it must be 'laconia', not
	// 'projects/my-proj/topics/laconia'.
	// It is optional. Defaults to 'cloud-builds' and the topic must be 'cloud-builds'
	// +optional
	Topic *string `json:"topic,omitempty"`
}

// CloudBuildSourceEventSource returns the Cloud Build CloudEvent source value.
func CloudBuildSourceEventSource(googleCloudProject, buildId string) string {
	return fmt.Sprintf("//cloudbuild.googleapis.com/projects/%s/builds/%s", googleCloudProject, buildId)
}

const (
	// CloudBuildSourceConditionReady has status True when the CloudBuildSource is
	// ready to send events.
	CloudBuildSourceConditionReady = apis.ConditionReady
)

var buildCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
)

// CloudBuildSourceStatus defines the observed state of CloudBuildSource.
type CloudBuildSourceStatus struct {
	duckv1alpha1.PubSubStatus `json:",inline"`
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
func (s *CloudBuildSource) IdentitySpec() *duckv1alpha1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudBuildSource) IdentityStatus() *duckv1alpha1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// CloudBuildSourceSpec returns the CloudBuildSourceSpec portion of the Spec.
func (bs *CloudBuildSource) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &bs.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (bs *CloudBuildSource) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &bs.Status.PubSubStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object.
func (bs *CloudBuildSource) ConditionSet() *apis.ConditionSet {
	return &buildCondSet
}
