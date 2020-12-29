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

package handler

import (
	"context"
	"net/http"
	"sync"

	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/zap"

	"cloud.google.com/go/pubsub"

	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/deliver"
	"github.com/google/knative-gcp/pkg/broker/handler/processors/filter"
	"github.com/google/knative-gcp/pkg/metrics"
)

// RetryPool is the sync pool for retry handlers.
// For each trigger in the config, it will attempt to create a handler.
// It will also stop/delete the handler if the corresponding trigger is deleted
// in the config.
type RetryPool struct {
	options *Options
	targets config.ReadonlyTargets
	pool    *syncMapTargetKey
	// Pubsub client used to pull events from decoupling topics.
	pubsubClient *pubsub.Client
	// For initial events delivery. We only need a shared client.
	// And we can set target address dynamically.
	deliverClient *http.Client
	statsReporter *metrics.DeliveryReporter
}

type retryHandlerCache struct {
	Handler
	t *config.Target
}

// If somehow the existing handler's setting has deviated from the current target config,
// we need to renew the handler.
func (hc *retryHandlerCache) shouldRenew(t *config.Target) bool {
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

// NewRetryPool creates a new retry handler pool.
func NewRetryPool(
	targets config.ReadonlyTargets,
	pubsubClient *pubsub.Client,
	deliverClient *http.Client,
	statsReporter *metrics.DeliveryReporter,
	opts ...Option) (*RetryPool, error) {
	options, err := NewOptions(opts...)
	if err != nil {
		return nil, err
	}

	p := &RetryPool{
		targets:       targets,
		options:       options,
		pool:          &syncMapTargetKey{},
		pubsubClient:  pubsubClient,
		deliverClient: deliverClient,
		statsReporter: statsReporter,
	}
	return p, nil
}

// SyncOnce syncs once the handler pool based on the targets config.
func (p *RetryPool) SyncOnce(ctx context.Context) error {
	ctx, err := p.statsReporter.AddTags(ctx)
	if err != nil {
		logging.FromContext(ctx).Error("failed to add tags to context", zap.Error(err))
	}

	p.pool.Range(func(key config.TargetKey, value *retryHandlerCache) bool {
		// Each target represents a trigger.
		if _, ok := p.targets.GetTargetByKey(&key); !ok {
			value.Stop()
			p.pool.Delete(key)
		}
		return true
	})

	p.targets.RangeAllTargets(func(t *config.Target) bool {
		if value, ok := p.pool.Load(*t.Key()); ok {
			// Skip if we don't need to renew the handler.
			if !value.shouldRenew(t) {
				return true
			}
			// Stop and clean up the old handler before we start a new one.
			value.Stop()
			p.pool.Delete(*t.Key())
		}

		// Don't start the handler if the target is not ready.
		// The retry topic/sub might not be ready at this point.
		if t.State != config.State_READY {
			return true
		}

		sub := p.pubsubClient.Subscription(t.RetryQueue.Subscription)
		sub.ReceiveSettings = p.options.PubsubReceiveSettings

		h := NewHandler(
			sub,
			processors.ChainProcessors(
				&filter.Processor{Targets: p.targets},
				&deliver.Processor{
					DeliverClient: p.deliverClient,
					Targets:       p.targets,
					StatsReporter: p.statsReporter,
				},
			),
			p.options.TimeoutPerEvent,
		)
		hc := &retryHandlerCache{
			Handler: *h,
			t:       t,
		}

		ctx, err := metrics.AddTargetTags(ctx, t)
		if err != nil {
			logging.FromContext(ctx).Error("failed to add target tags to context", zap.Error(err))
		}

		// Deliver processor needs the broker in the context for reply.
		ctx = handlerctx.WithBrokerKey(ctx, t.Key().ParentKey())
		ctx = handlerctx.WithTargetKey(ctx, t.Key())
		// Start the handler with target in context.
		hc.Start(ctx, func(err error) {
			// We will anyway get an error because of https://github.com/cloudevents/sdk-go/issues/470
			if err != nil {
				logging.FromContext(ctx).Error("handler for trigger has stopped with error", zap.Stringer("trigger", t.Key()), zap.Error(err))
			} else {
				logging.FromContext(ctx).Info("handler for trigger has stopped", zap.Stringer("trigger", t.Key()))
			}
		})

		p.pool.Store(*t.Key(), hc)
		return true
	})

	return nil
}

// syncMapTargetKey is a typed version of sync.Map.
type syncMapTargetKey struct {
	m sync.Map
}

func (m *syncMapTargetKey) Store(k config.TargetKey, v *retryHandlerCache) {
	m.m.Store(k, v)
}

func (m *syncMapTargetKey) Load(k config.TargetKey) (*retryHandlerCache, bool) {
	v, ok := m.m.Load(k)
	if v == nil {
		return nil, ok
	}
	return v.(*retryHandlerCache), ok
}

func (m *syncMapTargetKey) Delete(k config.TargetKey) {
	m.m.Delete(k)
}

func (m *syncMapTargetKey) Range(f func(key config.TargetKey, value *retryHandlerCache) bool) {
	wrapped := func(key interface{}, value interface{}) bool {
		var wrappedValue *retryHandlerCache
		if value != nil {
			wrappedValue = value.(*retryHandlerCache)
		}
		return f(key.(config.TargetKey), wrappedValue)
	}
	m.m.Range(wrapped)
}
