/*
Copyright 2017 The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Scheculer is a specification for a Scheculer resource
type Scheculer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheculerSpec   `json:"spec"`
	Status ScheculerStatus `json:"status"`
}

var (
	_ apis.Validatable   = (*Storage)(nil)
	_ apis.Defaultable   = (*Storage)(nil)
	_ runtime.Object     = (*Storage)(nil)
	_ kmeta.OwnerRefable = (*Storage)(nil)
	_ webhook.GenericCRD = (*Storage)(nil)
)

// ScheculerSpec is the spec for a Scheculer resource
type ScheculerSpec struct {
	// This brings in CloudEventOverrides and Sink
	duckv1beta1.SourceSpec

	// Secret is the credential to use to create the Scheduler Job.
	// If not specified, defaults to:
	// Name: google-cloud-key
	// Key: key.json
	// +optional
	Secret *corev1.SecretKeySelector `json:"Secret,omitempty"`

	// PubSubSecret is the credential to use to create
	// Topic / PullSubscription resources. If omitted, uses Secret
	PubSubSecret *corev1.SecretKeySelector `json:"pubsubSecret,omitempty"`

	// Project is the ID of the Google Cloud Project that the PubSub Topic exists in.
	// If omitted, defaults to same as the cluster.
	// +optional
	Project string `json:"project,omitempty"`

	// Location where to create the Job in.
	Location string `json:"location"`

	// Schedule in cron format, for example: "* * * * *" would be run
	// every minute.
	Schedule string `json:"schedule"`

	// What data to send
	// +optional
	Data string `json:"data,omitempty"`
}

// ScheculerStatus is the status for a Scheculer resource
type ScheculerStatus struct {
	// This brings in duck/v1beta1 Status as well as SinkURI
	duckv1beta1.SourceStatus

	// Job is the created scheduler Job on success
	// +optional
	Job string `json:"job,omitempty"`
}

func (scheduler *Scheduler) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Scheduler")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScheculerList is a list of Scheculer resources
type ScheculerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Scheculer `json:"items"`
}
