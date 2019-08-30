/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// GetCondition returns the condition currently associated with the given type,
// or nil.
func (ds *DecoratorStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return decoratorCondSet.Manage(ds).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ds *DecoratorStatus) IsReady() bool {
	return decoratorCondSet.Manage(ds).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ds *DecoratorStatus) InitializeConditions() {
	decoratorCondSet.Manage(ds).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (ds *DecoratorStatus) MarkSink(sink *apis.URL) {
	var uri string
	if sink != nil {
		uri = sink.String()
	}
	ds.SinkURI = uri
	if len(uri) > 0 {
		decoratorCondSet.Manage(ds).MarkTrue(DecoratorConditionSinkProvided)
	} else {
		decoratorCondSet.Manage(ds).MarkUnknown(DecoratorConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (ds *DecoratorStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	decoratorCondSet.Manage(ds).MarkFalse(DecoratorConditionSinkProvided, reason, messageFormat, messageA...)
}

// SetAddress updates the Addressable status of the Decorator and propagates a
// url status to the Addressable status condition based on url.
func (ds *DecoratorStatus) SetAddress(url *apis.URL) {
	if ds.Address == nil {
		ds.Address = &duckv1.Addressable{}
	}
	if url != nil {
		ds.Address.URL = url
		decoratorCondSet.Manage(ds).MarkTrue(DecoratorConditionAddressable)
	} else {
		ds.Address.URL = nil
		decoratorCondSet.Manage(ds).MarkFalse(DecoratorConditionAddressable, "emptyUrl", "url is empty")
	}
}

// MarkServiceReady sets the condition that the service has been created and ready.
func (ds *DecoratorStatus) MarkServiceReady() {
	decoratorCondSet.Manage(ds).MarkTrue(DecoratorConditionServiceReady)
}

// MarkServiceOperating sets the condition that the serivce is being created.
func (ds *DecoratorStatus) MarkServiceOperating(reason, messageFormat string, messageA ...interface{}) {
	decoratorCondSet.Manage(ds).MarkUnknown(DecoratorConditionServiceReady, reason, messageFormat, messageA...)
}

// MarkNoService sets the condition that signals there is not a service for this
// Decorator. This could be because of an error or the Decorator is being deleted.
func (ds *DecoratorStatus) MarkNoService(reason, messageFormat string, messageA ...interface{}) {
	decoratorCondSet.Manage(ds).MarkFalse(DecoratorConditionServiceReady, reason, messageFormat, messageA...)
}

// MarkServiceNotOwned sets the condition that signals there is an existing service that conflicts with
// the expected service name for this decorator.
func (ds *DecoratorStatus) MarkServiceNotOwned(messageFormat string, messageA ...interface{}) {
	decoratorCondSet.Manage(ds).MarkFalse(DecoratorConditionServiceReady, "NotOwned", messageFormat, messageA...)
}

func (ds *DecoratorStatus) PropagateServiceStatus(ready *apis.Condition) {
	switch {
	case ready == nil:
		ds.MarkServiceOperating("ServiceStatus", "Decorator Service has no Ready type status.")
	case ready.IsTrue():
		ds.MarkServiceReady()
	case ready.IsFalse():
		ds.MarkNoService(ready.Reason, ready.Message)
	}
}
