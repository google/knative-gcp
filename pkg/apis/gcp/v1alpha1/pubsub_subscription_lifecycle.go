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

var gcpPubSubSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	PubSubSubscriptionConditionSinkProvided,
	PubSubSubscriptionConditionDeployed,
	PubSubSubscriptionConditionSubscribed)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *PubSubStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return gcpPubSubSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *PubSubStatus) IsReady() bool {
	return gcpPubSubSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *PubSubStatus) InitializeConditions() {
	gcpPubSubSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *PubSubStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		gcpPubSubSourceCondSet.Manage(s).MarkTrue(PubSubSubscriptionConditionSinkProvided)
	} else {
		gcpPubSubSourceCondSet.Manage(s).MarkUnknown(PubSubSubscriptionConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *PubSubStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gcpPubSubSourceCondSet.Manage(s).MarkFalse(PubSubSubscriptionConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *PubSubStatus) MarkDeployed() {
	gcpPubSubSourceCondSet.Manage(s).MarkTrue(PubSubSubscriptionConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *PubSubStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	gcpPubSubSourceCondSet.Manage(s).MarkUnknown(PubSubSubscriptionConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *PubSubStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	gcpPubSubSourceCondSet.Manage(s).MarkFalse(PubSubSubscriptionConditionDeployed, reason, messageFormat, messageA...)
}

func (s *PubSubStatus) MarkSubscribed() {
	gcpPubSubSourceCondSet.Manage(s).MarkTrue(PubSubSubscriptionConditionSubscribed)
}

// MarkEventTypes sets the condition that the source has created its event types.
func (s *PubSubStatus) MarkEventTypes() {
	gcpPubSubSourceCondSet.Manage(s).MarkTrue(PubSubSubscriptionConditionEventTypesProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event types configured.
func (s *PubSubStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	gcpPubSubSourceCondSet.Manage(s).MarkFalse(PubSubSubscriptionConditionEventTypesProvided, reason, messageFormat, messageA...)
}
