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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Scheduler is a specification for a Scheduler resource
type Scheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulerSpec   `json:"spec"`
	Status SchedulerStatus `json:"status"`
}

var (
	_ apis.Validatable   = (*Storage)(nil)
	_ apis.Defaultable   = (*Storage)(nil)
	_ runtime.Object     = (*Storage)(nil)
	_ kmeta.OwnerRefable = (*Storage)(nil)
	_ webhook.GenericCRD = (*Storage)(nil)
)

// SchedulerSpec is the spec for a Scheduler resource
type SchedulerSpec struct {
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
	Data string `json:"data"`
}

const (
	// SchedulerConditionReady has status True when Scheduler is ready to send events.
	SchedulerConditionReady = apis.ConditionReady

	// JobReady has status True when Scheduler Job has been successfully created.
	JobReady apis.ConditionType = "JobReady"
)

var schedulerCondSet = apis.NewLivingConditionSet(
	PullSubscriptionReady,
	TopicReady,
	JobReady)

// SchedulerStatus is the status for a Scheduler resource
type SchedulerStatus struct {
	// This brings in duck/v1beta1 Status as well as SinkURI
	duckv1beta1.SourceStatus

	// JobName is the name of the created scheduler Job on success.
	// +optional
	JobName string `json:"jobName,omitempty"`

	// TopicID is the Topic used to deliver Scheduler events.
	// +optional
	TopicID string `json:"topicId,omitempty"`

	// ProjectID is the Project ID of the Topic used to deliver
	// Scheduler events.
	// +optional
	ProjectID string `json:"projectId,omitempty"`
}

func (scheduler *Scheduler) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Scheduler")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulerList is a list of Scheduler resources
type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Scheduler `json:"items"`
}
