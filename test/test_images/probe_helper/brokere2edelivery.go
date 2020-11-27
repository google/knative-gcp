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

package main

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
)

// Probe event goes here
const (
	namespaceExtension = "namespace"
)

type BrokerE2EDeliveryForwardProbe struct {
	ProbeInterface
	event     cloudevents.Event
	channelID string
	target    string
	ceClient  cloudevents.Client
}

func BrokerE2EDeliveryForwardProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	//requestHost, ok := event.Extensions()[ProbeEventRequestHostExtension]
	//if !ok {
	//	return nil, fmt.Errorf("Failed to read '%s' extension", ProbeEventRequestHostExtension)
	//}
	namespace, ok := event.Extensions()[namespaceExtension]
	if !ok {
		return nil, fmt.Errorf("Broker E2E delivery probe event has no '%s' extension", namespaceExtension)
	}
	probe := &BrokerE2EDeliveryForwardProbe{
		event:     event,
		channelID: event.ID(),
		target:    fmt.Sprintf("%s/%s/default", ph.brokerCellIngressBaseURL, namespace),
		ceClient:  ph.ceForwardClient,
	}
	return probe, nil
}

func (p BrokerE2EDeliveryForwardProbe) ChannelID() string {
	return p.channelID
}

func (p BrokerE2EDeliveryForwardProbe) Handle(ctx context.Context) error {
	// The probe client forwards the event to the broker `default` in the given namespace
	ctx = cecontext.WithTarget(ctx, p.target)
	if res := p.ceClient.Send(ctx, p.event); !cloudevents.IsACK(res) {
		return fmt.Errorf("Could not send event to broker target '%s', got result %s", p.target, res)
	}
	return nil
}

// Receiver event goes here
type BrokerE2EDeliveryReceiveProbe struct {
	ProbeInterface
	channelID string
}

func BrokerE2EDeliveryReceiveProbeConstructor(ph *ProbeHelper, event cloudevents.Event) (ProbeInterface, error) {
	// The event is received as sent.
	//
	// Example:
	//   Context Attributes,
	//     specversion: 1.0
	//     type: broker-e2e-delivery-probe
	//     source: probe
	//     id: 5950a1f1-f128-4c8e-bcdd-a2c96b4e5b78
	//     time: 2020-09-14T18:49:01.455945852Z
	//     datacontenttype: application/json
	//   Extensions,
	//     knativearrivaltime: 2020-09-14T18:49:01.455947424Z
	//     traceparent: 00-82b13494f5bcddc7b3007a7cd7668267-64e23f1193ceb1b7-00
	//   Data,
	//     { ... }
	return &BrokerE2EDeliveryReceiveProbe{
		channelID: event.ID(),
	}, nil
}

func (p BrokerE2EDeliveryReceiveProbe) ChannelID() string {
	return p.channelID
}
