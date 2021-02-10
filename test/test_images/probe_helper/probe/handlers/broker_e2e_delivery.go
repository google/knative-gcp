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
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

const (
	// BrokerE2EDeliveryProbeEventType is the CloudEvent type of broker e2e
	// delivery probes.
	BrokerE2EDeliveryProbeEventType = "broker-e2e-delivery-probe"

	brokerExtension    = "broker"
	namespaceExtension = "namespace"
)

func NewBrokerE2EDeliveryProbe(brokerCellIngressBaseURL string, client CeForwardClient) *BrokerE2EDeliveryProbe {
	return &BrokerE2EDeliveryProbe{
		brokerCellIngressBaseURL: brokerCellIngressBaseURL,
		client:                   client,
		receivedEvents:           utils.NewSyncReceivedEvents(),
	}
}

// BrokerE2EDeliveryProbe is the probe handler for probe requests in the broker
// e2e delivery probe.
type BrokerE2EDeliveryProbe struct {
	// The base URL for the BrokerCell Ingress
	brokerCellIngressBaseURL string

	// The client responsible for sending events to the BrokerCell Ingress
	client CeForwardClient

	// The map of received events to be tracked by the forwarder and receiver
	receivedEvents *utils.SyncReceivedEvents
}

// Forward sends an event to a given broker in a given namespace.
func (p *BrokerE2EDeliveryProbe) Forward(ctx context.Context, event cloudevents.Event) error {
	namespace, ok := event.Extensions()[namespaceExtension]
	if !ok {
		return fmt.Errorf("Broker e2e delivery probe event has no '%s' extension", namespaceExtension)
	}
	broker, ok := event.Extensions()[brokerExtension]
	if !ok {
		broker = "default"
	}

	// Create the receiver channel
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventTargetPathExtension]), event.ID())
	cleanupFunc, err := p.receivedEvents.CreateReceiverChannel(channelID)
	if err != nil {
		return fmt.Errorf("Failed to create receiver channel: %v", err)
	}
	defer cleanupFunc()

	// The probe sends the event to a given broker in a given namespace.
	target := fmt.Sprintf("%s/%s/%s", p.brokerCellIngressBaseURL, namespace, broker)
	ctx = cecontext.WithTarget(ctx, target)
	logging.FromContext(ctx).Infow("Sending event to broker target", zap.String("target", target))
	if res := p.client.Send(ctx, event); !cloudevents.IsACK(res) {
		return fmt.Errorf("Could not send event to broker target '%s', got result %s", target, res)
	}

	return p.receivedEvents.WaitOnReceiverChannel(ctx, channelID)
}

// Receive closes the receiver channel associated with a particular event.
func (p *BrokerE2EDeliveryProbe) Receive(ctx context.Context, event cloudevents.Event) error {
	// The event is received as sent.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: broker-e2e-delivery-probe
	//     source: probe
	//     id: broker-e2e-delivery-probe-5950a1f1-f128-4c8e-bcdd-a2c96b4e5b78
	//     time: 2020-09-14T18:49:01.455945852Z
	//     datacontenttype: application/json
	//   Extensions,
	//     knativearrivaltime: 2020-09-14T18:49:01.455947424Z
	//     traceparent: 00-82b13494f5bcddc7b3007a7cd7668267-64e23f1193ceb1b7-00
	//   Data,
	//     { ... }
	channelID := channelID(fmt.Sprint(event.Extensions()[utils.ProbeEventReceiverPathExtension]), event.ID())
	if err := p.receivedEvents.SignalReceiverChannel(channelID); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("Successfully received broker e2e delivery probe event")
	return nil
}
