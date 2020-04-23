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
			value.(*handler.Handler).Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeAllTargets(func(t *config.Target) bool {
		// There is already a handler for the trigger, skip.
		if _, ok := p.pool.Load(t.Key()); ok {
			return true
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

		h := &handler.Handler{
			Timeout:      p.options.TimeoutPerEvent,
			PubsubEvents: ps,
			Processor: processors.ChainProcessors(
				&filter.Processor{Targets: p.targets},
				&deliver.Processor{DeliverClient: p.deliverClient, Targets: p.targets},
			),
		}

		// Deliver processor needs the broker in the context for reply.
		tctx := handlerctx.WithBrokerKey(ctx, config.BrokerKey(t.Namespace, t.Broker))
		tctx = handlerctx.WithTargetKey(tctx, t.Key())
		// Start the handler with target in context.
		h.Start(tctx, func(err error) {
			if err != nil {
				logging.FromContext(ctx).Error("handler for trigger has stopped with error", zap.String("trigger", t.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for trigger has stopped", zap.String("trigger", t.Key()))
			}
			// Make sure the handler is deleted from the pool.
			p.pool.Delete(h)
		})

		p.pool.Store(t.Key(), h)
		return true
	})

	if errs > 0 {
		return fmt.Errorf("%d errors happened during handlers pool sync", errs)
	}

	return nil
}
