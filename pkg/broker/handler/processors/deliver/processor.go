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

package deliver

import (
	"context"
	"fmt"

	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"

	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

// Processor delivers events based on the broker/target in the context.
type Processor struct {
	processors.BaseProcessor

	// Requester is the cloudevents client to send events.
	Requester ceclient.Client
}

var _ processors.Interface = (*Processor)(nil)

// Process delivers the event based on the broker/target in the context.
func (p *Processor) Process(ctx context.Context, event *event.Event) error {
	broker, err := handlerctx.GetBroker(ctx)
	if err != nil {
		return err
	}
	target, err := handlerctx.GetTarget(ctx)
	if err != nil {
		return err
	}

	resp, res := p.Requester.Request(cecontext.WithTarget(ctx, target.Address), *event)
	if !protocol.IsACK(res) {
		// TODO(yolocs): Have option to send to retry.
		return fmt.Errorf("deliver event failed with error: %v", res.Error())
	}
	if resp == nil {
		return nil
	}

	if res := p.Requester.Send(cecontext.WithTarget(ctx, broker.Address), *resp); !protocol.IsACK(res) {
		return fmt.Errorf("deliver replied event failed with error: %v", res.Error())
	}

	return nil
}
