/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// PubSub is an Implementable "duck type".
var _ duck.Implementable = (*PubSub)(nil)

// +genduck
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSub is a shared type that GCP sources which create a
// Topic / PullSubscription will use.
// This duck type is intended to allow implementors of GCP sources
// which use PubSub for their transport.
type PubSub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PubSubSpec   `json:"spec"`
	Status PubSubStatus `json:"status"`
}

type PubSubSpec struct {
	// This brings in CloudEventOverrides and Sink.
	duckv1.SourceSpec `json:",inline"`

	IdentitySpec `json:",inline"`

	// Secret is the credential to use to poll from a Cloud Pub/Sub subscription.
	// If not specified, defaults to:
	// Name: google-cloud-key
	// Key: key.json
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Project is the ID of the Google Cloud Project that the PubSub Topic exists in.
	// If omitted, defaults to same as the cluster.
	// +optional
	Project string `json:"project,omitempty"`
}

// PubSubStatus shows how we expect folks to embed Addressable in
// their Status field.
type PubSubStatus struct {
	IdentityStatus `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the Source.
	// +optional
	SinkURI *apis.URL `json:"sinkUri,omitempty"`

	// CloudEventAttributes are the specific attributes that the Source uses
	// as part of its CloudEvents.
	// +optional
	CloudEventAttributes []duckv1.CloudEventAttributes `json:"ceAttributes,omitempty"`

	// ProjectID is the project ID of the Topic, might have been resolved.
	// +optional
	ProjectID string `json:"projectId,omitempty"`

	// TopicID where the notifications are sent to.
	// +optional
	TopicID string `json:"topicId,omitempty"`

	// SubscriptionID is the created subscription ID.
	// +optional
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

const (
	// TopicReady has status True when the PubSub Topic is ready.
	TopicReady apis.ConditionType = "TopicReady"

	// PullSubscriptionReay has status True when the PullSubscription is ready.
	PullSubscriptionReady apis.ConditionType = "PullSubscriptionReady"
)

var (
	// Verify PubSub resources meet duck contracts.
	_ duck.Populatable = (*PubSub)(nil)
	_ apis.Listable    = (*PubSub)(nil)
)

// GetFullType implements duck.Implementable
func (*PubSub) GetFullType() duck.Populatable {
	return &PubSub{}
}

// Populate implements duck.Populatable
func (s *PubSub) Populate() {
	s.Spec.Sink = duckv1.Destination{
		URI: &apis.URL{
			Scheme:   "https",
			Host:     "tableflip.dev",
			RawQuery: "flip=mattmoor",
		},
	}
	s.Spec.CloudEventOverrides = &duckv1.CloudEventOverrides{
		Extensions: map[string]string{"boosh": "kakow"},
	}
	s.Spec.Secret = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
		Key:                  "secretkey",
	}
	s.Status.ObservedGeneration = 42
	s.Status.Conditions = duckv1.Conditions{{
		// Populate ALL fields
		Type:               duckv1.SourceConditionSinkProvided,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Date(1984, 02, 28, 18, 52, 00, 00, time.UTC))},
	}}
	s.Status.SinkURI = &apis.URL{
		Scheme:   "https",
		Host:     "tableflip.dev",
		RawQuery: "flip=mattmoor",
	}
	s.Status.ProjectID = "projectid"
	s.Status.TopicID = "topicid"
	s.Status.SubscriptionID = "subscriptionid"
}

// GetListType implements apis.Listable
func (*PubSub) GetListType() runtime.Object {
	return &PubSubList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubList is a list of PubSub resources
type PubSubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PubSub `json:"items"`
}
