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

package fanout

import (
	"context"
	"fmt"
	"sync"

	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/handler"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/deliver"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/fanout"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/filter"
)

// SyncPool is the sync pool for fanout handlers.
// For each broker in the config, it will attempt to create a handler.
// It will also stop/delete the handler if the corresponding broker is deleted
// in the config.
type SyncPool struct {
	options *pool.Options
	targets config.ReadonlyTargets
	pool    sync.Map

	// For sending retry events. We only need a shared client.
	// And we can set retry topic dynamically.
	deliverRetryClient ceclient.Client
	// For initial events delivery. We only need a shared client.
	// And we can set target address dynamically.
	deliverClient ceclient.Client
}

type handlerCache struct {
	handler.Handler
	b *config.Broker
}

func (hc *handlerCache) requiresRestart(b *config.Broker) bool {
	if b == nil || b.DecoupleQueue == nil {
		return true
	}
	if b.DecoupleQueue.Topic != hc.b.DecoupleQueue.Topic ||
		b.DecoupleQueue.Subscription != hc.b.DecoupleQueue.Subscription {
		return true
	}
	return false
}

// NewSyncPool creates a new fanout handler pool.
func NewSyncPool(ctx context.Context, targets config.ReadonlyTargets, opts ...pool.Option) (*SyncPool, error) {
	options, err := pool.NewOptions(opts...)
	if err != nil {
		return nil, err
	}

	rps, err := defaultRetryPubsubProtocol(ctx, options)
	if err != nil {
		return nil, err
	}
	retryClient, err := ceclient.NewObserved(rps, options.CeClientOptions...)
	if err != nil {
		return nil, err
	}

	hp, err := cehttp.New()
	if err != nil {
		return nil, err
	}
	deliverClient, err := ceclient.NewObserved(hp, options.CeClientOptions...)
	if err != nil {
		return nil, err
	}

	p := &SyncPool{
		targets:            targets,
		options:            options,
		deliverClient:      deliverClient,
		deliverRetryClient: retryClient,
	}
	return p, nil
}

func defaultRetryPubsubProtocol(ctx context.Context, options *pool.Options) (*pubsub.Protocol, error) {
	opts := []pubsub.Option{
		pubsub.WithProjectID(options.ProjectID),
	}
	if options.PubsubClient != nil {
		opts = append(opts, pubsub.WithClient(options.PubsubClient))
	}
	return pubsub.New(ctx, opts...)
}

// SyncOnce syncs once the handler pool based on the targets config.
func (p *SyncPool) SyncOnce(ctx context.Context) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		if _, ok := p.targets.GetBrokerByKey(key.(string)); !ok {
			value.(*handlerCache).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeBrokers(func(b *config.Broker) bool {
		// There is already a handler for the broker, skip.
		if value, ok := p.pool.Load(b.Key()); ok {
			// Skip if we don't need to restart the handler.
			if !value.(*handlerCache).requiresRestart(b) {
				return true
			}
			// Stop and clean up the old handler before we start a new one.
			value.(*handlerCache).Stop()
			p.pool.Delete(b.Key())
		}

		opts := []pubsub.Option{
			pubsub.WithProjectID(p.options.ProjectID),
			pubsub.WithTopicID(b.DecoupleQueue.Topic),
			pubsub.WithSubscriptionID(b.DecoupleQueue.Subscription),
			pubsub.WithReceiveSettings(&p.options.PubsubReceiveSettings),
		}

		if p.options.PubsubClient != nil {
			opts = append(opts, pubsub.WithClient(p.options.PubsubClient))
		}
		ps, err := pubsub.New(ctx, opts...)
		if err != nil {
			logging.FromContext(ctx).Error("failed to create pubsub protocol", zap.String("broker", b.Key()), zap.Error(err))
			errs++
			return true
		}

		hc := &handlerCache{
			Handler: handler.Handler{
				Timeout:      p.options.TimeoutPerEvent,
				PubsubEvents: ps,
				Processor: processors.ChainProcessors(
					&fanout.Processor{MaxConcurrency: p.options.MaxConcurrencyPerEvent, Targets: p.targets},
					&filter.Processor{Targets: p.targets},
					&deliver.Processor{
						DeliverClient:      p.deliverClient,
						Targets:            p.targets,
						RetryOnFailure:     true,
						DeliverRetryClient: p.deliverRetryClient,
					},
				),
			},
			b: b,
		}
		// Start the handler with broker key in context.
		hc.Start(handlerctx.WithBrokerKey(ctx, b.Key()), func(err error) {
			if err != nil {
				logging.FromContext(ctx).Error("handler for broker has stopped with error", zap.String("broker", b.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for broker has stopped", zap.String("broker", b.Key()))
			}
			// Make sure the handler is deleted from the pool.
			p.pool.Delete(b.Key())
		})

		p.pool.Store(b.Key(), hc)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
