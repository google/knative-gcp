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

package probe

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
)

const (
	// BrokerE2EDeliveryProbeEventType is the CloudEvent type of forward broker e2e
	// delivery probes.
	BrokerE2EDeliveryProbeEventType = "broker-e2e-delivery-probe"

	namespaceExtension = "namespace"
)

// BrokerE2EDeliveryForwardProbe is the probe handler for forward probe requests
// in the broker e2e delivery probe.
type BrokerE2EDeliveryForwardProbe struct {
	Handler
	event     cloudevents.Event
	channelID string
	target    string
	ceClient  cloudevents.Client
}

// BrokerE2EDeliveryForwardProbeConstructor builds a new broker e2e delivery forward
// probe handler from a given CloudEvent.
func BrokerE2EDeliveryForwardProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
	namespace, ok := event.Extensions()[namespaceExtension]
	if !ok {
		return nil, fmt.Errorf("Broker e2e delivery probe event has no '%s' extension", namespaceExtension)
	}
	probe := &BrokerE2EDeliveryForwardProbe{
		event:     event,
		channelID: fmt.Sprintf("%s/%s", requestHost, event.ID()),
		target:    fmt.Sprintf("%s/%s/default", ph.BrokerCellIngressBaseURL, namespace),
		ceClient:  ph.CeForwardClient,
	}
	return probe, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p BrokerE2EDeliveryForwardProbe) ChannelID() string {
	return p.channelID
}

// Handle sends an event to the broker named 'default' in a given namespace.
func (p BrokerE2EDeliveryForwardProbe) Handle(ctx context.Context) error {
	ctx = cecontext.WithTarget(ctx, p.target)
	if res := p.ceClient.Send(ctx, p.event); !cloudevents.IsACK(res) {
		return fmt.Errorf("Could not send event to broker target '%s', got result %s", p.target, res)
	}
	return nil
}

// BrokerE2EDeliveryReceiveProbe is the probe handler for receiver probe requests
// in the broker e2e delivery probe.
type BrokerE2EDeliveryReceiveProbe struct {
	Handler
	channelID string
}

// BrokerE2EDeliveryReceiveProbeConstructor builds a new broker e2e delivery receiver
// probe handler from a given CloudEvent.
func BrokerE2EDeliveryReceiveProbeConstructor(ph *Helper, event cloudevents.Event, requestHost string) (Handler, error) {
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
	return &BrokerE2EDeliveryReceiveProbe{
		channelID: fmt.Sprintf("%s/%s", requestHost, event.ID()),
	}, nil
}

// ChannelID returns the unique channel ID for a given probe request.
func (p BrokerE2EDeliveryReceiveProbe) ChannelID() string {
	return p.channelID
}
