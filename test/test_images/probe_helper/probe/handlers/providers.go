/*
Copyright 2020 Google LLC

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

package handlers

import (
	"github.com/google/wire"

	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
)

var HandlerSet wire.ProviderSet = wire.NewSet(
	wire.Bind(new(Interface), new(*EventTypeProbe)),
	NewEventTypeHandler,
	utils.NewSyncReceivedEvents,
	NewBrokerE2EDeliveryProbe,
	NewCloudAuditLogsSourceProbe,
	NewApiServerSourceProbe,
	wire.Struct(new(ApiServerSourceCreateProbe), "*"),
	wire.Struct(new(ApiServerSourceUpdateProbe), "*"),
	wire.Struct(new(ApiServerSourceDeleteProbe), "*"),
	NewCloudPubSubSourceProbe,
	NewCloudSchedulerSourceProbe,
	NewPingSourceProbe,
	NewCloudStorageSourceProbe,
	wire.Struct(new(CloudStorageSourceCreateProbe), "*"),
	wire.Struct(new(CloudStorageSourceDeleteProbe), "*"),
	wire.Struct(new(CloudStorageSourceArchiveProbe), "*"),
	wire.Struct(new(CloudStorageSourceUpdateMetadataProbe), "*"),
	NewLivenessChecker,
)

func NewLivenessChecker(cloudSchedulerProbe *CloudSchedulerSourceProbe, pingSourceProbe *PingSourceProbe) *utils.LivenessChecker {
	return &utils.LivenessChecker{ActionFuncs: []utils.ActionFunc{
		cloudSchedulerProbe.CleanupStaleSchedulerTimes(),
		pingSourceProbe.CleanupStalePingSourceTimes(),
	}}
}
