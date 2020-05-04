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
	"time"

	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/eventutil"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

// Processor delivers events based on the broker/target in the context.
type Processor struct {
	processors.BaseProcessor

	// DeliverClient is the cloudevents client to send events.
	DeliverClient ceclient.Client

	// Targets is the targets from config.
	Targets config.ReadonlyTargets

	// RetryOnFailure if set to true, the processor will send the event
	// to the retry topic if the delivery fails.
	RetryOnFailure bool

	// DeliverRetryClient is the cloudevents client to send events
	// to the retry topic.
	DeliverRetryClient ceclient.Client

	// DeliverTimeout is the timeout applied to cancel delivery.
	// If zero, not additional timeout is applied.
	DeliverTimeout time.Duration

	// EventTTL manages events TTL.
	EventTTL *eventutil.TTL
}

var _ processors.Interface = (*Processor)(nil)

// Process delivers the event based on the broker/target in the context.
func (p *Processor) Process(ctx context.Context, event *event.Event) error {
	bk, err := handlerctx.GetBrokerKey(ctx)
	if err != nil {
		return err
	}
	tk, err := handlerctx.GetTargetKey(ctx)
	if err != nil {
		return err
	}
	broker, ok := p.Targets.GetBrokerByKey(bk)
	if !ok {
		// If the broker no longer exists, then there is nothing to process.
		logging.FromContext(ctx).Warn("broker no longer exist in the config", zap.String("broker", bk))
		return nil
	}
	target, ok := p.Targets.GetTargetByKey(tk)
	if !ok {
		// If the target no longer exists, then there is nothing to process.
		logging.FromContext(ctx).Warn("target no longer exist in the config", zap.String("target", tk))
		return nil
	}

	// TTL is a broker local counter so remove any TTL before forwarding.
	// Do not modify the original event as we need to send the original
	// event to retry queue on failure.
	copy := event.Clone()
	ttl, ttlFound := p.EventTTL.GetTTL(&copy)
	p.EventTTL.DeleteTTL(&copy)

	dctx := ctx
	if p.DeliverTimeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(dctx, p.DeliverTimeout)
		defer cancel()
	}

	// Forward the event copy that has TTL removed.
	resp, res := p.DeliverClient.Request(cecontext.WithTarget(dctx, target.Address), copy)
	if !protocol.IsACK(res) {
		if !p.RetryOnFailure {
			return fmt.Errorf("target delivery failed: %v", res.Error())
		}

		logging.FromContext(ctx).Warn("target delivery failed", zap.String("target", tk), zap.String("error", res.Error()))
		return p.sendToRetryTopic(ctx, target, event)
	}
	if resp == nil {
		return nil
	}

	// Attach the previous TTL for the reply.
	if ttlFound {
		// Clean up potential TTL from the reply.
		// It really shouldn't happen though.
		p.EventTTL.DeleteTTL(resp)
		p.EventTTL.UpdateTTL(resp, ttl)
	}

	if res := p.DeliverClient.Send(cecontext.WithTarget(dctx, broker.Address), *resp); !protocol.IsACK(res) {
		if !p.RetryOnFailure {
			return fmt.Errorf("delivery of replied event failed: %v", res.Error())
		}

		logging.FromContext(ctx).Warn("delivery of replied event failed", zap.String("target", tk), zap.String("error", res.Error()))
		return p.sendToRetryTopic(ctx, target, event)
	}

	// For post-delivery processing.
	return p.Next().Process(ctx, event)
}

func (p *Processor) sendToRetryTopic(ctx context.Context, target *config.Target, event *event.Event) error {
	pctx := cecontext.WithTopic(ctx, target.RetryQueue.Topic)
	if err := p.DeliverRetryClient.Send(pctx, *event); err != nil {
		return fmt.Errorf("failed to send event to retry topic: %w", err)
	}
	return nil
}
