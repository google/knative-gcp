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
	options        *pool.Options
	targetsWatcher config.TargetsWatcher
	pool           sync.Map
}

// StartSyncPool starts the sync pool.
func StartSyncPool(ctx context.Context, targetsWatcher config.TargetsWatcher, opts ...pool.Option) (*SyncPool, error) {
	p := &SyncPool{
		targetsWatcher: targetsWatcher,
		options:        pool.NewOptions(opts...),
	}
	go p.watch(ctx)
	return p, nil
}

func (p *SyncPool) watch(ctx context.Context) {
	for {
		targets := p.targetsWatcher.Targets()
		if err := p.syncTargets(ctx, targets.Config); err != nil {
			logging.FromContext(ctx).Error("failed to sync handlers pool on watch signal", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-targets.Updated:
		}
	}
}

// BrokerKey returns the key of a broker.
func brokerKey(namespace, name string) string {
	return namespace + "/" + name
}

func (p *SyncPool) syncTargets(ctx context.Context, targets *config.TargetsConfig) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		if _, ok := targets.Brokers[key.(string)]; !ok {
			value.(*handler.Handler).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	for _, b := range targets.Brokers {
		bk := brokerKey(b.Namespace, b.Name)

		// Update config of existing handler
		if h, ok := p.pool.Load(bk); ok {
			h.(*handler.Handler).ConfigCh <- b
			continue
		}

		opts := []pubsub.Option{
			pubsub.WithTopicID(b.DecoupleQueue.Topic),
			pubsub.WithSubscriptionID(b.DecoupleQueue.Subscription),
			pubsub.WithReceiveSettings(&p.options.PubsubReceiveSettings),
		}

		if p.options.ProjectID != "" {
			opts = append(opts, pubsub.WithProjectID(p.options.ProjectID))
		} else {
			opts = append(opts, pubsub.WithProjectIDFromDefaultEnv())
		}

		if p.options.PubsubClient != nil {
			opts = append(opts, pubsub.WithClient(p.options.PubsubClient))
		}
		ps, err := pubsub.New(ctx, opts...)
		if err != nil {
			logging.FromContext(ctx).Error("failed to create pubsub protocol", zap.String("broker", bk), zap.Error(err))
			errs++
			continue
		}

		h := &handler.Handler{
			Timeout:      p.options.TimeoutPerEvent,
			PubsubEvents: ps,
			Processor: processors.ChainProcessors(
				&fanout.Processor{MaxConcurrency: p.options.MaxConcurrencyPerEvent},
				&filter.Processor{},
				&deliver.Processor{Requester: p.options.EventRequester},
			),
			ConfigCh: make(chan *config.Broker),
		}
		h.Start(ctx, b, func(err error) {
			if err != nil {
				logging.FromContext(ctx).Error("handler for broker has stopped with error", zap.String("broker", bk), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for broker has stopped", zap.String("broker", bk))
			}
			// Make sure the handler is deleted from the pool.
			p.pool.Delete(h)
		})

		p.pool.Store(bk, h)
	}

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
