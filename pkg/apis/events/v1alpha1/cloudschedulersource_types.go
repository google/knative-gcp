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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	kngcpduck "github.com/google/knative-gcp/pkg/duck"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudSchedulerSource is a specification for a CloudSchedulerSource resource
type CloudSchedulerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudSchedulerSourceSpec   `json:"spec"`
	Status CloudSchedulerSourceStatus `json:"status"`
}

const (
	// CloudEvent types used by CloudSchedulerSource.
	CloudSchedulerSourceExecute = "com.google.cloud.scheduler.job.execute"
	// CloudSchedulerSourceJobName is the Pub/Sub message attribute key with the CloudSchedulerSource's job name.
	CloudSchedulerSourceJobName = "jobName"
	// CloudSchedulerSourceName is the Pub/Sub message attribute key with the CloudSchedulerSource's name.
	CloudSchedulerSourceName = "schedulerName"
)

func CloudSchedulerSourceEventSource(parent, scheduler string) string {
	return fmt.Sprintf("//cloudscheduler.googleapis.com/%s/schedulers/%s", parent, scheduler)
}

var (
	_ kmeta.OwnerRefable           = (*CloudSchedulerSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudSchedulerSource)(nil)
	_ kngcpduck.PubSubable         = (*CloudSchedulerSource)(nil)
	_ kngcpduck.Identifiable       = (*CloudSchedulerSource)(nil)
	_                              = duck.VerifyType(&CloudSchedulerSource{}, &duckv1.Conditions{})
)

// CloudSchedulerSourceSpec is the spec for a CloudSchedulerSource resource
type CloudSchedulerSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	// Sink, CloudEventOverrides, Secret, PubSubSecret, and Project
	duckv1alpha1.PubSubSpec `json:",inline"`

	// Location where to create the Job in.
	Location string `json:"location"`

	// Schedule in cron format, for example: "* * * * *" would be run
	// every minute.
	Schedule string `json:"schedule"`

	// What data to send
	Data string `json:"data"`
}

const (
	// CloudSchedulerSourceConditionReady has status True when CloudSchedulerSource is ready to send events.
	CloudSchedulerSourceConditionReady = apis.ConditionReady

	// JobReady has status True when CloudSchedulerSource Job has been successfully created.
	JobReady apis.ConditionType = "JobReady"
)

var schedulerCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
	duckv1alpha1.TopicReady,
	JobReady)

// CloudSchedulerSourceStatus is the status for a CloudSchedulerSource resource
type CloudSchedulerSourceStatus struct {
	// This brings in our GCP PubSub based events importers
	// duck/v1beta1 Status, SinkURI, ProjectID, TopicID, and SubscriptionID
	duckv1alpha1.PubSubStatus `json:",inline"`

	// JobName is the name of the created scheduler Job on success.
	// +optional
	JobName string `json:"jobName,omitempty"`
}

func (scheduler *CloudSchedulerSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CloudSchedulerSource")
}

// Methods for identifiable interface
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *CloudSchedulerSource) IdentitySpec() *duckv1alpha1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudSchedulerSource) IdentityStatus() *duckv1alpha1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (s *CloudSchedulerSource) ConditionSet() *apis.ConditionSet {
	return &schedulerCondSet
}

// Methods for pubsubable interface
// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *CloudSchedulerSource) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *CloudSchedulerSource) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudSchedulerSourceList is a list of CloudSchedulerSource resources
type CloudSchedulerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CloudSchedulerSource `json:"items"`
}
