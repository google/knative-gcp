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
	"knative.dev/pkg/apis"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *PullSubscriptionStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pullSubscriptionCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *PullSubscriptionStatus) IsReady() bool {
	return pullSubscriptionCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *PullSubscriptionStatus) InitializeConditions() {
	pullSubscriptionCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *PullSubscriptionStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		pullSubscriptionCondSet.Manage(s).MarkTrue(PullSubscriptionConditionSinkProvided)
	} else {
		pullSubscriptionCondSet.Manage(s).MarkUnknown(PullSubscriptionConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *PullSubscriptionStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkFalse(PullSubscriptionConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkTransformer sets the condition that the source has a transformer configured.
func (s *PullSubscriptionStatus) MarkTransformer(uri string) {
	s.TransformerURI = uri
	if len(uri) > 0 {
		pullSubscriptionCondSet.Manage(s).MarkTrue(PullSubscriptionConditionTransformerProvided)
	} else {
		pullSubscriptionCondSet.Manage(s).MarkUnknown(PullSubscriptionConditionTransformerProvided, "TransformerEmpty", "Transformer has resolved to empty.")
	}
}

// MarkNoTransformer sets the condition that the source does not have a transformer configured.
func (s *PullSubscriptionStatus) MarkNoTransformer(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkFalse(PullSubscriptionConditionTransformerProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *PullSubscriptionStatus) MarkDeployed() {
	pullSubscriptionCondSet.Manage(s).MarkTrue(PullSubscriptionConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *PullSubscriptionStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkUnknown(PullSubscriptionConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *PullSubscriptionStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkFalse(PullSubscriptionConditionDeployed, reason, messageFormat, messageA...)
}

// MarkSubscribed sets the condition that the subscription has been created.
func (s *PullSubscriptionStatus) MarkSubscribed() {
	pullSubscriptionCondSet.Manage(s).MarkTrue(PullSubscriptionConditionSubscribed)
}

// MarkSubscriptionOperation sets the condition that the subscription is being operated on.
func (s *PullSubscriptionStatus) MarkSubscriptionOperation(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkUnknown(PullSubscriptionConditionSubscribed, reason, messageFormat, messageA...)
}

// MarkNoSubscription sets the condition that the subscription does not exist.
func (s *PullSubscriptionStatus) MarkNoSubscription(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkFalse(PullSubscriptionConditionSubscribed, reason, messageFormat, messageA...)
}

// MarkEventTypes sets the condition that the source has created its event types.
func (s *PullSubscriptionStatus) MarkEventTypes() {
	pullSubscriptionCondSet.Manage(s).MarkTrue(PullSubscriptionConditionEventTypesProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event types configured.
func (s *PullSubscriptionStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	pullSubscriptionCondSet.Manage(s).MarkFalse(PullSubscriptionConditionEventTypesProvided, reason, messageFormat, messageA...)
}
