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
	// Secret is the credential to use to create, publish, and poll the Pub/Sub
	// Topic and Subscriptions. The value of the secret entry must be a
	// service account key in the JSON format
	// (see https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the Pub/Sub
	// Topic and Subscriptions will be created in.
	Project string `json:"project,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount that will be used to
	// run the Channel Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Channel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

var channelCondSet = apis.NewLivingConditionSet(
	ChannelConditionServiceReady,
	ChannelConditionEndpointsReady,
	ChannelConditionAddressable,
	ChannelConditionChannelServiceReady,
	ChannelConditionTopicReady,
)

const (
	// ChannelConditionReady has status True when all subconditions below have
	// been set to True.
	ChannelConditionReady = apis.ConditionReady

	// ChannelConditionAddressable has status true when this Channel meets the
	// Addressable contract and has a non-empty hostname.
	ChannelConditionAddressable apis.ConditionType = "Addressable"

	// ChannelConditionServiceReady has status True when a k8s Service is
	// ready. This basically just means it exists because there's no meaningful
	// status in Service. See Endpoints below.
	ChannelConditionServiceReady apis.ConditionType = "ServiceReady"

	// ChannelConditionEndpointsReady has status True when a k8s Service
	// Endpoints are backed by at least one endpoint.
	ChannelConditionEndpointsReady apis.ConditionType = "EndpointsReady"

	// ChannelConditionServiceReady has status True when a k8s Service
	// representing the channel is ready. Because this uses ExternalName,
	// there are no endpoints to check.
	ChannelConditionChannelServiceReady apis.ConditionType = "ChannelServiceReady"

	// ChannelConditionTopicReady has status True when the Channel has had a
	// Pub/Sub topic created for it.
	ChannelConditionTopicReady apis.ConditionType = "TopicDeployed"

	// ChannelConditionInvokerDeployed has status True when the Channel has had
	// its invoker deployment created.
	ChannelConditionInvokerDeployed apis.ConditionType = "InvokerDeployed"
)

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

	// ProjectID is the resolved project ID in use by the PullSubscription.
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// TopicID is the created topic ID used by the Channel.
	// +optional
	TopicID string `json:"topicId,omitempty"`
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
