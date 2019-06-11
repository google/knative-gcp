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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetGroupVersionKind returns the GroupVersionKind.
func (s *PubSubSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("PubSubSource")
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *PubSubSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return pubSubSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *PubSubSourceStatus) IsReady() bool {
	return pubSubSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *PubSubSourceStatus) InitializeConditions() {
	pubSubSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *PubSubSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		pubSubSourceCondSet.Manage(s).MarkTrue(PubSubSourceConditionSinkProvided)
	} else {
		pubSubSourceCondSet.Manage(s).MarkUnknown(PubSubSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *PubSubSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	pubSubSourceCondSet.Manage(s).MarkFalse(PubSubSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkTransformer sets the condition that the source has a transformer configured.
func (s *PubSubSourceStatus) MarkTransformer(uri string) {
	s.TransformerURI = uri
	if len(uri) > 0 {
		pubSubSourceCondSet.Manage(s).MarkTrue(PubSubSourceConditionTransformerProvided)
	} else {
		pubSubSourceCondSet.Manage(s).MarkUnknown(PubSubSourceConditionTransformerProvided, "TransformerEmpty", "Transformer has resolved to empty.")
	}
}

// MarkNoTransformer sets the condition that the source does not have a transformer configured.
func (s *PubSubSourceStatus) MarkNoTransformer(reason, messageFormat string, messageA ...interface{}) {
	pubSubSourceCondSet.Manage(s).MarkFalse(PubSubSourceConditionTransformerProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *PubSubSourceStatus) MarkDeployed() {
	pubSubSourceCondSet.Manage(s).MarkTrue(PubSubSourceConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *PubSubSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	pubSubSourceCondSet.Manage(s).MarkUnknown(PubSubSourceConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *PubSubSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	pubSubSourceCondSet.Manage(s).MarkFalse(PubSubSourceConditionDeployed, reason, messageFormat, messageA...)
}

//
//func (s *PubSubSourceStatus) MarkSubscribed() {
//	pubSubSourceCondSet.Manage(s).MarkTrue(PubSubSourceConditionSubscribed)
//}

// MarkEventTypes sets the condition that the source has created its event types.
func (s *PubSubSourceStatus) MarkEventTypes() {
	pubSubSourceCondSet.Manage(s).MarkTrue(PubSubSourceConditionEventTypesProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event types configured.
func (s *PubSubSourceStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	pubSubSourceCondSet.Manage(s).MarkFalse(PubSubSourceConditionEventTypesProvided, reason, messageFormat, messageA...)
}
