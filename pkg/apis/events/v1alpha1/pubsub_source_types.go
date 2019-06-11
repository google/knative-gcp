/*
Copyright 2019 Google LLC

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
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubSource is the Schema for the gcppubsubsources API.
// +k8s:openapi-gen=true
type PubSubSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PubSubSourceSpec   `json:"spec,omitempty"`
	Status PubSubSourceStatus `json:"status,omitempty"`
}

// Check that PubSubSource can be validated and can be defaulted.
var _ runtime.Object = (*PubSubSource)(nil)

// Check that PubSubSource will be checked for immutable fields.
var _ apis.Immutable = (*PubSubSource)(nil)

// Check that PubSubSource implements the Conditions duck type.
var _ = duck.VerifyType(&PubSubSource{}, &duckv1alpha1.Conditions{})

// PubSubSourceSpec defines the desired state of the PubSubSource.
type PubSubSourceSpec struct {
	// Secret is the credential to use to poll the PubSubSource Subscription. It is not used
	// to create or delete the Subscription, only to poll it. The value of the secret entry must be
	// a service account key in the JSON format (see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys).
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// TODO: https://github.com/googleapis/google-cloud-go/blob/master/compute/metadata/metadata.go
	// Project is the ID of the Google Cloud Project that the PubSubSource Topic exists in.
	Project string `json:"project,omitempty"`

	// Topic is the ID of the PubSubSource Topic to Subscribe to. It must be in the form of the
	// unique identifier within the project, not the entire name. E.g. it must be 'laconia', not
	// 'projects/my-eventing-project/topics/laconia'.
	Topic string `json:"topic,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// Transformer is a reference to an object that will resolve to a domain name to use as the transformer.
	// +optional
	Transformer *corev1.ObjectReference `json:"transformer,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// PubSubEventType is the GcpPubSub CloudEvent type, in case PubSubSource doesn't send a
	// CloudEvent itself.
	PubSubEventType = "google.pubsub.topic.publish"
)

// PubSubEventSource returns the GcpPubSub CloudEvent source value.
func PubSubEventSource(googleCloudProject, topic string) string {
	return fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", googleCloudProject, topic)
}

const (
	// PubSubSourceConditionReady has status True when the PubSubSource is ready to send events.
	PubSubSourceConditionReady = duckv1alpha1.ConditionReady

	// PubSubSourceConditionSinkProvided has status True when the PubSubSource has been configured with a sink target.
	PubSubSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// PubSubSourceConditionDeployed has status True when the PubSubSource has had it's receive adapter deployment created.
	PubSubSourceConditionDeployed duckv1alpha1.ConditionType = "Deployed"

	// TODO: clean up after we are sure the controller does not control pub/sub.
	// PubSubSourceConditionSubscribed has status True when a GCP PubSubSource Subscription has been created pointing at the created receive adapter deployment.
	//PubSubSourceConditionSubscribed duckv1alpha1.ConditionType = "Subscribed"

	// PubSubSourceConditionTransformerProvided has status True when the PubSubSource has been configured with a transformer target.
	PubSubSourceConditionTransformerProvided duckv1alpha1.ConditionType = "TransformerProvided"

	// PubSubSourceConditionEventTypesProvided has status True when the PubSubSource has been configured with event types.
	PubSubSourceConditionEventTypesProvided duckv1alpha1.ConditionType = "EventTypesProvided"
)

var pubSubSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	PubSubSourceConditionSinkProvided,
	PubSubSourceConditionDeployed,

//PubSubSourceConditionSubscribed,
)

// PubSubSourceStatus defines the observed state of PubSubSource.
type PubSubSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the PubSubSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`

	// TransformerURI is the current active transformer URI that has been configured for the PubSubSource.
	// +optional
	TransformerURI string `json:"transformerUri,omitempty"`

	// ProjectID is the resolved project ID in use for the PubSubSource.
	// +optional
	ProjectID string `json:"projectID,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PubSubSourceList contains a list of PubSubs.
type PubSubSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PubSubSource `json:"items"`
}
