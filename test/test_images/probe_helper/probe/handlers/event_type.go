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
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/pkg/logging"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

// EventTypeProbe is a handler that maps an event types to its corresponding underlying handler.
type EventTypeProbe struct {
	forward map[string]Interface
	receive map[string]Interface
}

func NewEventTypeHandler(brokerE2EDeliveryProbe *BrokerE2EDeliveryProbe, cloudPubSubSourceProbe *CloudPubSubSourceProbe,
	cloudStorageSourceCreateProbe *CloudStorageSourceCreateProbe, cloudStorageSourceUpdateMetadataProbe *CloudStorageSourceUpdateMetadataProbe,
	cloudStorageSourceArchiveProbe *CloudStorageSourceArchiveProbe, cloudStorageSourceDeleteProbe *CloudStorageSourceDeleteProbe,
	cloudAuditLogsSourceProbe *CloudAuditLogsSourceProbe, cloudSchedulerSourceProbe *CloudSchedulerSourceProbe) *EventTypeProbe {
	// Set the forward and receiver probe handlers now that they are initialized.
	forwardHandlers := map[string]Interface{
		BrokerE2EDeliveryProbeEventType:                brokerE2EDeliveryProbe,
		CloudPubSubSourceProbeEventType:                cloudPubSubSourceProbe,
		CloudStorageSourceCreateProbeEventType:         cloudStorageSourceCreateProbe,
		CloudStorageSourceUpdateMetadataProbeEventType: cloudStorageSourceUpdateMetadataProbe,
		CloudStorageSourceArchiveProbeEventType:        cloudStorageSourceArchiveProbe,
		CloudStorageSourceDeleteProbeEventType:         cloudStorageSourceDeleteProbe,
		CloudAuditLogsSourceProbeEventType:             cloudAuditLogsSourceProbe,
		CloudSchedulerSourceProbeEventType:             cloudSchedulerSourceProbe,
	}
	receiveHandlers := map[string]Interface{
		BrokerE2EDeliveryProbeEventType:                      brokerE2EDeliveryProbe,
		schemasv1.CloudPubSubMessagePublishedEventType:       cloudPubSubSourceProbe,
		schemasv1.CloudStorageObjectFinalizedEventType:       cloudStorageSourceCreateProbe,
		schemasv1.CloudStorageObjectMetadataUpdatedEventType: cloudStorageSourceUpdateMetadataProbe,
		schemasv1.CloudStorageObjectArchivedEventType:        cloudStorageSourceArchiveProbe,
		schemasv1.CloudStorageObjectDeletedEventType:         cloudStorageSourceDeleteProbe,
		schemasv1.CloudAuditLogsLogWrittenEventType:          cloudAuditLogsSourceProbe,
		schemasv1.CloudSchedulerJobExecutedEventType:         cloudSchedulerSourceProbe,
	}
	return &EventTypeProbe{
		forward: forwardHandlers,
		receive: receiveHandlers,
	}
}

func (p *EventTypeProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	// Retrieve the probe handler based on the event type
	inner, ok := p.forward[event.Type()]
	if !ok {
		logging.FromContext(ctx).Warnw("Probe forwarding failed, unrecognized forward probe type")
		return cloudevents.ResultNACK
	}
	return inner.Forward(ctx, event)
}

func (p *EventTypeProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// Retrieve the probe handler based on the event type
	inner, ok := p.receive[event.Type()]
	if !ok {
		logging.FromContext(ctx).Warnw("Probe receiving failed, unrecognized receive probe type")
		return cloudevents.ResultNACK
	}
	return inner.Receive(ctx, event)
}
