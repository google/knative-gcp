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

package retry

import (
	"context"
	"fmt"
	"sync"

	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"

	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/handler"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/deliver"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/filter"
)

// SyncPool is the sync pool for retry handlers.
// For each trigger in the config, it will attempt to create a handler.
// It will also stop/delete the handler if the corresponding trigger is deleted
// in the config.
type SyncPool struct {
	options *pool.Options
	targets config.ReadonlyTargets
	pool    sync.Map
	// For initial events delivery. We only need a shared client.
	// And we can set target address dynamically.
	deliverClient ceclient.Client
}

type handlerCache struct {
	handler.Handler
	t *config.Target
}

// If somehow the existing handler's setting has deviated from the current target config,
// we need to renew the handler.
func (hc *handlerCache) shouldRenew(t *config.Target) bool {
	if !hc.IsAlive() {
		return true
	}
	// If this really happens, it means a data corruption.
	// The handler creation will fail (which is expected).
	if t == nil || t.RetryQueue == nil {
		return true
	}
	if t.RetryQueue.Topic != hc.t.RetryQueue.Topic ||
		t.RetryQueue.Subscription != hc.t.RetryQueue.Subscription {
		return true
	}
	return false
}

// NewSyncPool creates a new retry handler pool.
func NewSyncPool(targets config.ReadonlyTargets, opts ...pool.Option) (*SyncPool, error) {
	options, err := pool.NewOptions(opts...)
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
		targets:       targets,
		options:       options,
		deliverClient: deliverClient,
	}
	return p, nil
}

// SyncOnce syncs once the handler pool based on the targets config.
func (p *SyncPool) SyncOnce(ctx context.Context) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		// Each target represents a trigger.
		if _, ok := p.targets.GetTargetByKey(key.(string)); !ok {
			value.(*handlerCache).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeAllTargets(func(t *config.Target) bool {
		if value, ok := p.pool.Load(t.Key()); ok {
			// Skip if we don't need to renew the handler.
			if !value.(*handlerCache).shouldRenew(t) {
				return true
			}
			// Stop and clean up the old handler before we start a new one.
			value.(*handlerCache).Stop()
			p.pool.Delete(t.Key())
		}

		opts := []pubsub.Option{
			pubsub.WithProjectID(p.options.ProjectID),
			pubsub.WithTopicID(t.RetryQueue.Topic),
			pubsub.WithSubscriptionID(t.RetryQueue.Subscription),
			pubsub.WithReceiveSettings(&p.options.PubsubReceiveSettings),
		}

		if p.options.PubsubClient != nil {
			opts = append(opts, pubsub.WithClient(p.options.PubsubClient))
		}
		ps, err := pubsub.New(ctx, opts...)
		if err != nil {
			logging.FromContext(ctx).Error("failed to create pubsub protocol", zap.String("trigger", t.Key()), zap.Error(err))
			errs++
			return true
		}

		hc := &handlerCache{
			Handler: handler.Handler{
				Timeout:      p.options.TimeoutPerEvent,
				PubsubEvents: ps,
				Processor: processors.ChainProcessors(
					&filter.Processor{Targets: p.targets},
					&deliver.Processor{DeliverClient: p.deliverClient, Targets: p.targets},
				),
			},
			t: t,
		}

		// Deliver processor needs the broker in the context for reply.
		tctx := handlerctx.WithBrokerKey(ctx, config.BrokerKey(t.Namespace, t.Broker))
		tctx = handlerctx.WithTargetKey(tctx, t.Key())
		// Start the handler with target in context.
		hc.Start(tctx, func(err error) {
			// We will anyway get an error because of https://github.com/cloudevents/sdk-go/issues/470
			if err != nil {
				logging.FromContext(ctx).Error("handler for trigger has stopped with error", zap.String("trigger", t.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for trigger has stopped", zap.String("trigger", t.Key()))
			}
		})

		p.pool.Store(t.Key(), hc)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
