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
	"sync/atomic"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
	"github.com/google/knative-gcp/pkg/metrics"
)

type fanoutResult struct {
	targetKey string
	err       error
}

// Processor fanouts an event based on the broker key in the context.
type Processor struct {
	processors.BaseProcessor

	// MaxConcurrency is the max number of goroutine it will spawn
	// for each event.
	MaxConcurrency int

	// Targets is the targets from config.
	Targets config.ReadonlyTargets
}

var _ processors.Interface = (*Processor)(nil)

// Process fanouts the given event.
func (p *Processor) Process(ctx context.Context, event *event.Event) error {
	bk, err := handlerctx.GetBrokerKey(ctx)
	if err != nil {
		return err
	}
	broker, ok := p.Targets.GetBrokerByKey(bk)
	if !ok {
		// If the broker no longer exists, then there is nothing to process.
		logging.FromContext(ctx).Warn("broker no longer exist in the config", zap.String("broker", bk))
		return nil
	}

	tc := make(chan *config.Target)
	go func() {
		defer close(tc)
		for _, target := range broker.Targets {
			tc <- target
		}
	}()

	curr := len(broker.Targets)
	if curr > p.MaxConcurrency {
		curr = p.MaxConcurrency
	}

	resChs := make([]<-chan *fanoutResult, 0, curr)
	for i := 0; i < curr; i++ {
		resChs = append(resChs, p.fanoutEvent(ctx, event, tc))
	}

	return p.mergeResults(ctx, resChs)
}

func (p *Processor) fanoutEvent(ctx context.Context, event *event.Event, tc <-chan *config.Target) <-chan *fanoutResult {
	out := make(chan *fanoutResult)
	go func() {
		defer close(out)
		for target := range tc {
			// Timeout is controller by the context.
			ctx, err := metrics.AddTargetTags(ctx, target)
			if err != nil {
				logging.FromContext(ctx).Error(
					"failed to add trigger name tag to context",
					zap.String("trigger", target.Name),
				)
			}
			ctx = handlerctx.WithTargetKey(ctx, target.Key())
			out <- &fanoutResult{
				targetKey: target.Key(),
				err:       p.Next().Process(ctx, event),
			}
		}
	}()
	return out
}

func (p *Processor) mergeResults(ctx context.Context, resChs []<-chan *fanoutResult) error {
	bk, err := handlerctx.GetBrokerKey(ctx)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var errs, passes int32

	count := func(c <-chan *fanoutResult) {
		for fr := range c {
			if fr.err != nil {
				logging.FromContext(ctx).Error("error processing event for fanout target", zap.String("target", fr.targetKey))
				atomic.AddInt32(&errs, 1)
			} else {
				atomic.AddInt32(&passes, 1)
			}
		}
		wg.Done()
	}

	wg.Add(len(resChs))
	for _, c := range resChs {
		go count(c)
	}
	wg.Wait()

	if errs > 0 {
		return fmt.Errorf("event fanout passed %d targets, failed %d targets", passes, errs)
	}

	logging.FromContext(ctx).Debug("event fanout successful", zap.String("broker", bk), zap.Int32("count", passes))
	return nil
}
