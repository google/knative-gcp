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
}

func NewSyncPool(targets config.ReadonlyTargets, opts ...pool.Option) (*SyncPool, error) {
	options, err := pool.NewOptions(opts...)
	if err != nil {
		return nil, err
	}
	p := &SyncPool{
		targets: targets,
		options: options,
	}
	return p, nil
}

func (p *SyncPool) SyncOnce(ctx context.Context) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		if _, ok := p.targets.GetBrokerByKey(key.(string)); !ok {
			value.(*handler.Handler).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeBrokers(func(b *config.Broker) bool {
		// There is already a handler for the broker, skip.
		if _, ok := p.pool.Load(b.Key()); ok {
			return true
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

		h := &handler.Handler{
			Timeout:      p.options.TimeoutPerEvent,
			PubsubEvents: ps,
			Processor: processors.ChainProcessors(
				&fanout.Processor{MaxConcurrency: p.options.MaxConcurrencyPerEvent, Targets: p.targets},
				&filter.Processor{Targets: p.targets},
				&deliver.Processor{Requester: p.options.EventRequester, Targets: p.targets},
			),
		}
		// Start the handler with broker key in context.
		h.Start(handlerctx.WithBrokerKey(ctx, b.Key()), func(err error) {
			if err != nil {
				logging.FromContext(ctx).Error("handler for broker has stopped with error", zap.String("broker", b.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for broker has stopped", zap.String("broker", b.Key()))
			}
			// Make sure the handler is deleted from the pool.
			p.pool.Delete(h)
		})

		p.pool.Store(b.Key(), h)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
