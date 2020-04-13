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

	"go.uber.org/zap"

	"knative.dev/pkg/logging"

	"github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/handler"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/deliver"
)

// TODO Retry and Fanout are using similar SyncPool struct, needing an interface to reduce redundancies.
// SyncPool is the sync pool for retry handlers.
// For each trigger in the config, it will attempt to create a handler.
// It will also stop/delete the handler if the corresponding trigger is deleted
// in the config.
type SyncPool struct {
	options *pool.Options
	targets config.ReadonlyTargets
	pool    sync.Map
}

// StartSyncPool starts the sync pool.
func StartSyncPool(ctx context.Context, targets config.ReadonlyTargets, opts ...pool.Option) (*SyncPool, error) {
	p := &SyncPool{
		targets: targets,
		options: pool.NewOptions(opts...),
	}
	if err := p.syncOnce(ctx); err != nil {
		return nil, err
	}
	if p.options.SyncSignal != nil {
		go p.watch(ctx)
	}
	return p, nil
}

func (p *SyncPool) watch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.options.SyncSignal:
			if err := p.syncOnce(ctx); err != nil {
				logging.FromContext(ctx).Error("failed to sync handlers pool on watch signal", zap.Error(err))
			}
		}
	}
}

func (p *SyncPool) syncOnce(ctx context.Context) error {
	var errs int

	p.pool.Range(func(key, value interface{}) bool {
		// Each target represents a trigger.
		if _, ok := p.targets.GetTargetByKey(key.(string)); !ok {
			value.(*handler.Handler).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeAllTargets(func(t *config.Target) bool {
		tk := config.TriggerKey(t.Namespace, t.Broker, t.Name)

		// There is already a handler for the trigger, skip.
		if _, ok := p.pool.Load(tk); ok {
			return true
		}

		projectIDOpt := pubsub.WithProjectIDFromDefaultEnv()
		if p.options.ProjectID != "" {
			projectIDOpt = pubsub.WithProjectID(p.options.ProjectID)
		}
		ps, err := pubsub.New(ctx,
			projectIDOpt,
			pubsub.WithTopicID(t.RetryQueue.Topic),
			pubsub.WithSubscriptionID(t.RetryQueue.Subscription),
		)
		if err != nil {
			logging.FromContext(ctx).Error("failed to create pubsub protocol", zap.String("trigger", t.Name), zap.Error(err))
			errs++
			return true
		}

		h := &handler.Handler{
			Timeout:      p.options.TimeoutPerEvent,
			PubsubEvents: ps,
			Processor: processors.ChainProcessors(
				// TODO filter processor may be added in the future, but need more discussion for that.
				&deliver.Processor{Requester: p.options.EventRequester},
			),
		}
		// Start the handler with target in context.
		h.Start(handlerctx.WithTarget(ctx, t), func(err error) {
			if err != nil {
				logging.FromContext(ctx).Error("handler for broker has stopped with error", zap.String("trigger", t.Name), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for broker has stopped", zap.String("trigger", t.Name))
			}
			// Make sure the handler is deleted from the pool.
			p.pool.Delete(h)
		})

		p.pool.Store(tk, h)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
