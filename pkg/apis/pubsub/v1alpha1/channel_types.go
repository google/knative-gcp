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
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	"github.com/knative/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
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
var _ apis.Validatable = (*Channel)(nil)
var _ apis.Defaultable = (*Channel)(nil)
var _ runtime.Object = (*Channel)(nil)
var _ webhook.GenericCRD = (*Channel)(nil)

// ChannelSpec defines which subscribers have expressed interest in
// receiving events from this Channel.
// arguments for a Channel.
type ChannelSpec struct {
	// Secret is the credential to use to create and poll the PubSubSource
	// Subscription. The value of the secret entry must be a service account
	// key in the JSON format (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the PubSubSource
	// Topic exists in.
	Project string `json:"project,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to
	// run the Receive Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Channel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

// ChannelStatus represents the current state of a Channel.
type ChannelStatus struct {
	// inherits duck/v1beta1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// Channel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1alpha1.AddressStatus `json:",inline"`

	// Subscribers is populated with the statuses of each of the Channelable's subscribers.
	eventingduck.SubscribableStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a collection of in-memory channels.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for PubSubChannels
func (c *Channel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Channel")
}
