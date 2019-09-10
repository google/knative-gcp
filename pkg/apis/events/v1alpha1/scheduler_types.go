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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
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
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1alpha1.PubSubSpec

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
	duckv1alpha1.PullSubscriptionReady,
	duckv1alpha1.TopicReady,
	JobReady)

// SchedulerStatus is the status for a Scheduler resource
type SchedulerStatus struct {
	// This brings in our GCP PubSub based events importers
	// duck/v1beta1 Status, SinkURI, ProjectID, TopicID, and SubscriptionID
	duckv1alpha1.PubSubStatus

	// JobName is the name of the created scheduler Job on success.
	// +optional
	JobName string `json:"jobName,omitempty"`
}

func (scheduler *Scheduler) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Scheduler")
}

// Methods for pubsubable interface
// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *Scheduler) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *Scheduler) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *Scheduler) ConditionSet() *apis.ConditionSet {
	return &StorageCondSet
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulerList is a list of Scheduler resources
type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Scheduler `json:"items"`
}
