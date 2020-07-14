/*
Copyright 2020 Google LLC.

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
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	kngcpduck "github.com/google/knative-gcp/pkg/duck/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudAuditLogsSource is a specification for a Cloud Audit Log event source.
type CloudAuditLogsSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudAuditLogsSourceSpec   `json:"spec"`
	Status CloudAuditLogsSourceStatus `json:"status"`
}

// Verify that CloudAuditLogsSource matches various duck types.
var (
	_ apis.Defaultable             = (*CloudAuditLogsSource)(nil)
	_ apis.Validatable             = (*CloudAuditLogsSource)(nil)
	_ kmeta.OwnerRefable           = (*CloudAuditLogsSource)(nil)
	_ resourcesemantics.GenericCRD = (*CloudAuditLogsSource)(nil)
	_ kngcpduck.Identifiable       = (*CloudAuditLogsSource)(nil)
	_ kngcpduck.PubSubable         = (*CloudAuditLogsSource)(nil)
	_ duckv1.KRShaped              = (*CloudAuditLogsSource)(nil)
)

const (
	SinkReady apis.ConditionType = "SinkReady"
)

var auditLogsSourceCondSet = apis.NewLivingConditionSet(
	gcpduckv1.PullSubscriptionReady,
	gcpduckv1.TopicReady,
	SinkReady,
)

type CloudAuditLogsSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	gcpduckv1.PubSubSpec `json:",inline"`

	// The CloudAuditLogsSource will pull events matching the following
	// parameters:

	// The GCP service providing audit logs. Required.
	ServiceName string `json:"serviceName"`
	// The name of the service method or operation. For API calls,
	// this should be the name of the API method. Required.
	MethodName string `json:"methodName"`
	// The resource or collection that is the target of the
	// operation. The name is a scheme-less URI, not including the
	// API service name.
	ResourceName string `json:"resourceName,omitempty"`
}

type CloudAuditLogsSourceStatus struct {
	gcpduckv1.PubSubStatus `json:",inline"`

	// ID of the Stackdriver sink used to publish audit log messages.
	StackdriverSink string `json:"stackdriverSink,omitempty"`
}

func (*CloudAuditLogsSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CloudAuditLogsSource")
}

// Methods for identifiable interface.
// IdentitySpec returns the IdentitySpec portion of the Spec.
func (s *CloudAuditLogsSource) IdentitySpec() *gcpduckv1.IdentitySpec {
	return &s.Spec.IdentitySpec
}

// IdentityStatus returns the IdentityStatus portion of the Status.
func (s *CloudAuditLogsSource) IdentityStatus() *gcpduckv1.IdentityStatus {
	return &s.Status.IdentityStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object.
func (*CloudAuditLogsSource) ConditionSet() *apis.ConditionSet {
	return &auditLogsSourceCondSet
}

///Methods for pubsubable interface.

// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *CloudAuditLogsSource) PubSubSpec() *gcpduckv1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *CloudAuditLogsSource) PubSubStatus() *gcpduckv1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CloudAuditLogsSourceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []CloudAuditLogsSource `json:"items"`
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*CloudAuditLogsSource) GetConditionSet() apis.ConditionSet {
	return auditLogsSourceCondSet
}

// GetStatus retrieves the status of the CloudAuditLogsSource. Implements the KRShaped interface.
func (s *CloudAuditLogsSource) GetStatus() *duckv1.Status {
	return &s.Status.Status
}
