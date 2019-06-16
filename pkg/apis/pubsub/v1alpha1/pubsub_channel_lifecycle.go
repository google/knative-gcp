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
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck/v1alpha1"
)

var pubsubChannelCondSet = apis.NewLivingConditionSet(
	PubSubChannelConditionServiceReady,
	PubSubChannelConditionEndpointsReady,
	PubSubChannelConditionAddressable,
	PubSubChannelConditionChannelServiceReady,
)

const (
	// PubSubChannelConditionReady has status True when all subconditions below have been set to True.
	PubSubChannelConditionReady = apis.ConditionReady

	// PubSubChannelConditionAddressable has status true when this PubSubChannel meets
	// the Addressable contract and has a non-empty hostname.
	PubSubChannelConditionAddressable apis.ConditionType = "Addressable"

	// PubSubChannelConditionServiceReady has status True when a k8s Service is ready. This
	// basically just means it exists because there's no meaningful status in Service. See Endpoints
	// below.
	PubSubChannelConditionServiceReady apis.ConditionType = "ServiceReady"

	// PubSubChannelConditionEndpointsReady has status True when a k8s Service Endpoints are backed
	// by at least one endpoint.
	PubSubChannelConditionEndpointsReady apis.ConditionType = "EndpointsReady"

	// PubSubChannelConditionServiceReady has status True when a k8s Service representing the channel is ready.
	// Because this uses ExternalName, there are no endpoints to check.
	PubSubChannelConditionChannelServiceReady apis.ConditionType = "ChannelServiceReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (cs *PubSubChannelStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pubsubChannelCondSet.Manage(cs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (cs *PubSubChannelStatus) IsReady() bool {
	return pubsubChannelCondSet.Manage(cs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (cs *PubSubChannelStatus) InitializeConditions() {
	pubsubChannelCondSet.Manage(cs).InitializeConditions()
}

// TODO: Use the new beta duck types.
func (cs *PubSubChannelStatus) SetAddress(url *apis.URL) {
	if cs.Address == nil {
		cs.Address = &v1alpha1.Addressable{}
	}
	if url != nil {
		cs.Address.Hostname = url.Host
		cs.Address.URL = url
		pubsubChannelCondSet.Manage(cs).MarkTrue(PubSubChannelConditionAddressable)
	} else {
		cs.Address.Hostname = ""
		cs.Address.URL = nil
		pubsubChannelCondSet.Manage(cs).MarkFalse(PubSubChannelConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}

func (cs *PubSubChannelStatus) MarkServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	pubsubChannelCondSet.Manage(cs).MarkFalse(PubSubChannelConditionServiceReady, reason, messageFormat, messageA...)
}

func (cs *PubSubChannelStatus) MarkServiceTrue() {
	pubsubChannelCondSet.Manage(cs).MarkTrue(PubSubChannelConditionServiceReady)
}

func (cs *PubSubChannelStatus) MarkChannelServiceFailed(reason, messageFormat string, messageA ...interface{}) {
	pubsubChannelCondSet.Manage(cs).MarkFalse(PubSubChannelConditionChannelServiceReady, reason, messageFormat, messageA...)
}

func (cs *PubSubChannelStatus) MarkChannelServiceTrue() {
	pubsubChannelCondSet.Manage(cs).MarkTrue(PubSubChannelConditionChannelServiceReady)
}

func (cs *PubSubChannelStatus) MarkEndpointsFailed(reason, messageFormat string, messageA ...interface{}) {
	pubsubChannelCondSet.Manage(cs).MarkFalse(PubSubChannelConditionEndpointsReady, reason, messageFormat, messageA...)
}

func (cs *PubSubChannelStatus) MarkEndpointsTrue() {
	pubsubChannelCondSet.Manage(cs).MarkTrue(PubSubChannelConditionEndpointsReady)
}
