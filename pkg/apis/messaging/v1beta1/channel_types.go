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

package v1beta1

import (
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	kngcpduckv1 "github.com/google/knative-gcp/pkg/duck/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is a resource representing an channel backed by Google Cloud Pub/Sub.
type Channel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec ChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the Channel. This data may be out of
	// date.
	// +optional
	Status ChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var (
	_ apis.Convertible             = (*Channel)(nil)
	_ apis.Defaultable             = (*Channel)(nil)
	_ apis.Validatable             = (*Channel)(nil)
	_ runtime.Object               = (*Channel)(nil)
	_ resourcesemantics.GenericCRD = (*Channel)(nil)
	_ kngcpduckv1.Identifiable     = (*Channel)(nil)
	_ duckv1.KRShaped              = (*Channel)(nil)
)

// ChannelSpec defines which subscribers have expressed interest in
// receiving events from this Channel.
// arguments for a Channel.
type ChannelSpec struct {
	gcpduckv1.IdentitySpec `json:",inline"`
	// Secret is the credential to use to create, publish, and poll the Pub/Sub
	// Topic and Subscriptions. The value of the secret entry must be a
	// service account key in the JSON format
	// (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the Pub/Sub
	// Topic and Subscriptions will be created in.
	// +optional
	Project string `json:"project,omitempty"`

	// Channel conforms to Duck type Subscribable.
	// +optional
	*eventingduck.SubscribableSpec `json:",inline"`
}

var channelCondSet = apis.NewLivingConditionSet(
	ChannelConditionAddressable,
	ChannelConditionTopicReady,
)

const (
	// ChannelConditionReady has status True when all subconditions below have
	// been set to True.
	ChannelConditionReady = apis.ConditionReady

	// ChannelConditionAddressable has status true when this Channel meets the
	// Addressable contract and has a non-empty url.
	ChannelConditionAddressable apis.ConditionType = "Addressable"

	// ChannelConditionTopicReady has status True when the Channel has had a
	// Pub/Sub topic created for it.
	ChannelConditionTopicReady apis.ConditionType = "TopicReady"
)

// ChannelStatus represents the current state of a Channel.
type ChannelStatus struct {
	gcpduckv1.IdentityStatus `json:",inline"`

	// Channel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1.AddressStatus `json:",inline"`

	// SubscribableStatus is populated with the statuses of each of the Channelable's subscribers.
	eventingduck.SubscribableStatus `json:",inline"`

	// ProjectID is the resolved project ID in use by the Channel.
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// TopicID is the created topic ID used by the Channel.
	// +optional
	TopicID string `json:"topicId,omitempty"`
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (c *Channel) IdentitySpec() *gcpduckv1.IdentitySpec {
	return &c.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (c *Channel) IdentityStatus() *gcpduckv1.IdentityStatus {
	return &c.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *Channel) ConditionSet() *apis.ConditionSet {
	return &channelCondSet
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a collection of Pub/Sub backed Channels.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Pub/Sub backed Channel.
func (c *Channel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Channel")
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*Channel) GetConditionSet() apis.ConditionSet {
	return channelCondSet
}

// GetStatus retrieves the status of the Channel. Implements the KRShaped interface.
func (c *Channel) GetStatus() *duckv1.Status {
	return &c.Status.Status
}
