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

package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/handler"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/deliver"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/fanout"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/filter"
)

// FanoutPool is the sync pool for fanout handlers.
// For each broker in the config, it will attempt to create a handler.
// It will also stop/delete the handler if the corresponding broker is deleted
// in the config.
type FanoutPool struct {
	options *Options
	targets config.ReadonlyTargets
	pool    sync.Map

	// Pubsub client used to pull events from decoupling topics.
	pubsubClient *pubsub.Client
	// For sending retry events. We only need a shared client.
	// And we can set retry topic dynamically.
	deliverRetryClient ceclient.Client
	// For initial events delivery. We only need a shared client.
	// And we can set target address dynamically.
	deliverClient ceclient.Client
	// For fanout delivery, we need a slightly shorter timeout
	// than the handler timeout per event.
	// It allows the delivery processor to timeout the delivery
	// before the handler nacks the pubsub message, which will
	// cause event re-delivery for all targets.
	deliverTimeout time.Duration
}

type fanoutHandlerCache struct {
	handler.Handler
	b *config.Broker
}

// If somehow the existing handler's setting has deviated from the current broker config,
// we need to renew the handler.
func (hc *fanoutHandlerCache) shouldRenew(b *config.Broker) bool {
	if !hc.IsAlive() {
		return true
	}
	// If this really happens, it means a data corruption.
	// The handler creation will fail (which is expected).
	if b == nil || b.DecoupleQueue == nil {
		return true
	}
	if b.DecoupleQueue.Topic != hc.b.DecoupleQueue.Topic ||
		b.DecoupleQueue.Subscription != hc.b.DecoupleQueue.Subscription {
		return true
	}
	return false
}

// NewFanoutPool creates a new fanout handler pool.
func NewFanoutPool(
	targets config.ReadonlyTargets,
	pubsubClient *pubsub.Client,
	deliverClient DeliverClient,
	retryClient RetryClient,
	opts ...Option,
) (*FanoutPool, error) {
	options, err := NewOptions(opts...)
	if err != nil {
		return nil, err
	}

	if options.TimeoutPerEvent < 5*time.Second {
		return nil, fmt.Errorf("timeout per event cannot be lower than %v", 5*time.Second)
	}

	p := &FanoutPool{
		targets:            targets,
		options:            options,
		pubsubClient:       pubsubClient,
		deliverClient:      deliverClient,
		deliverRetryClient: retryClient,
		// Set the deliver timeout slightly less than the total timeout for each event.
		deliverTimeout: options.TimeoutPerEvent - (5 * time.Second),
	}
	return p, nil
}

// SyncOnce syncs once the handler pool based on the targets config.
func (p *FanoutPool) SyncOnce(ctx context.Context) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		if _, ok := p.targets.GetBrokerByKey(key.(string)); !ok {
			value.(*fanoutHandlerCache).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeBrokers(func(b *config.Broker) bool {
		if value, ok := p.pool.Load(b.Key()); ok {
			// Skip if we don't need to renew the handler.
			if !value.(*fanoutHandlerCache).shouldRenew(b) {
				return true
			}
			// Stop and clean up the old handler before we start a new one.
			value.(*fanoutHandlerCache).Stop()
			p.pool.Delete(b.Key())
		}

		sub := p.pubsubClient.Subscription(b.DecoupleQueue.Subscription)
		sub.ReceiveSettings = p.options.PubsubReceiveSettings

		hc := &fanoutHandlerCache{
			Handler: handler.Handler{
				Timeout:      p.options.TimeoutPerEvent,
				Subscription: sub,
				Processor: processors.ChainProcessors(
					&fanout.Processor{MaxConcurrency: p.options.MaxConcurrencyPerEvent, Targets: p.targets},
					&filter.Processor{Targets: p.targets},
					&deliver.Processor{
						DeliverClient:      p.deliverClient,
						Targets:            p.targets,
						RetryOnFailure:     true,
						DeliverRetryClient: p.deliverRetryClient,
						DeliverTimeout:     p.deliverTimeout,
					},
				),
			},
			b: b,
		}
		// Start the handler with broker key in context.
		hc.Start(handlerctx.WithBrokerKey(ctx, b.Key()), func(err error) {
			// We will anyway get an error because of https://github.com/cloudevents/sdk-go/issues/470
			if err != nil {
				logging.FromContext(ctx).Error("handler for broker has stopped with error", zap.String("broker", b.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for broker has stopped", zap.String("broker", b.Key()))
			}
		})

		p.pool.Store(b.Key(), hc)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
