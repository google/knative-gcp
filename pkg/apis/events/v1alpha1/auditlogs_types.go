/*
Copyright 2019 Google LLC.

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
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/duck"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditLogsSource is a specification for a Cloud Audit Log event source.
type AuditLogsSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuditLogsSourceSpec   `json:"spec"`
	Status AuditLogsSourceStatus `json:"status"`
}

var (
	_ apis.Defaultable   = (*AuditLogsSource)(nil)
	_ runtime.Object     = (*AuditLogsSource)(nil)
	_ kmeta.OwnerRefable = (*AuditLogsSource)(nil)
	_ duck.PubSubable    = (*AuditLogsSource)(nil)
)

const (
	SinkReady apis.ConditionType = "SinkReady"
)

var auditLogsSourceCondSet = apis.NewLivingConditionSet(
	duckv1alpha1.PullSubscriptionReady,
	duckv1alpha1.TopicReady,
	SinkReady,
)

type AuditLogsSourceSpec struct {
	// This brings in the PubSub based Source Specs. Includes:
	duckv1alpha1.PubSubSpec `json:",inline"`

	// The AuditLogsSource will pull events matching the following
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

type AuditLogsSourceStatus struct {
	duckv1alpha1.PubSubStatus `json:",inline"`

	// ID of the Stackdriver sink used to publish audit log messages.
	SinkID string `json:"sinkId,omitempty"`
}

func (*AuditLogsSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("AuditLogsSource")
}

///Methods for pubsubable interface

// PubSubSpec returns the PubSubSpec portion of the Spec.
func (s *AuditLogsSource) PubSubSpec() *duckv1alpha1.PubSubSpec {
	return &s.Spec.PubSubSpec
}

// PubSubStatus returns the PubSubStatus portion of the Status.
func (s *AuditLogsSource) PubSubStatus() *duckv1alpha1.PubSubStatus {
	return &s.Status.PubSubStatus
}

// ConditionSet returns the apis.ConditionSet of the embedding object
func (*AuditLogsSource) ConditionSet() *apis.ConditionSet {
	return &auditLogsSourceCondSet
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AuditLogsSourceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []AuditLogsSource `json:"items"`
}
