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
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/eventutil"
	handlertesting "github.com/google/knative-gcp/pkg/broker/handler/testing"
	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"

	_ "knative.dev/pkg/metrics/testing"
)

const (
	retryPod       = "retry-pod"
	retryContainer = "retry-container"
)

func TestRetryWatchAndSync(t *testing.T) {
	reportertest.ResetDeliveryMetrics()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testProject := "test-project"
	helper, err := handlertesting.NewHelper(ctx, testProject)
	if err != nil {
		t.Fatalf("failed to create pool testing helper: %v", err)
	}
	defer helper.Close()

	signal := make(chan struct{})
	syncPool, err := InitializeTestRetryPool(helper.Targets, retryPod, retryContainer, helper.PubsubClient)
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	p, err := GetFreePort()
	if err != nil {
		t.Fatalf("failed to get random free port: %v", err)
	}

	t.Run("start sync pool creates no handler", func(t *testing.T) {
		_, err = StartSyncPool(ctx, syncPool, signal, time.Minute, p)
		if err != nil {
			t.Errorf("unexpected error from starting sync pool: %v", err)
		}
		assertRetryHandlers(t, syncPool, helper.Targets)
	})

	t.Run("no handler created for not-ready target", func(t *testing.T) {
		b := helper.GenerateBroker(ctx, t, "ns")
		target := helper.GenerateTarget(ctx, t, b.Key(), nil)
		target.State = config.State_UNKNOWN
		helper.Targets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
			bm.UpsertTargets(target)
		})
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertRetryHandlers(t, syncPool, helper.Targets)
	})

	bs := make([]*config.Broker, 0, 4)

	t.Run("adding some brokers with their targets", func(t *testing.T) {
		// Add some brokers with their targets.
		for i := 0; i < 4; i++ {
			b := helper.GenerateBroker(ctx, t, "ns")
			helper.GenerateTarget(ctx, t, b.Key(), nil)
			bs = append(bs, b)
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertRetryHandlers(t, syncPool, helper.Targets)
	})

	t.Run("delete and adding targets in brokers", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			for _, bt := range bs[i].Targets {
				helper.DeleteTarget(ctx, t, bt.Key())
				helper.GenerateTarget(ctx, t, bs[i].Key(), nil)
			}
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertRetryHandlers(t, syncPool, helper.Targets)
	})

	t.Run("deleting all brokers with their targets", func(t *testing.T) {
		// clean up all brokers
		for _, b := range bs {
			helper.DeleteBroker(ctx, t, b.Key())
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertRetryHandlers(t, syncPool, helper.Targets)
	})
}

func TestRetrySyncPoolE2E(t *testing.T) {
	reportertest.ResetDeliveryMetrics()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testProject := "test-project"

	helper, err := handlertesting.NewHelper(ctx, testProject)
	if err != nil {
		t.Fatalf("failed to create pool testing helper: %v", err)
	}
	defer helper.Close()

	// Create two brokers.
	b1 := helper.GenerateBroker(ctx, t, "ns")
	b2 := helper.GenerateBroker(ctx, t, "ns")

	t1 := helper.GenerateTarget(ctx, t, b1.Key(), nil)
	t2 := helper.GenerateTarget(ctx, t, b1.Key(), nil)
	t3 := helper.GenerateTarget(ctx, t, b2.Key(), nil)

	expectMetrics := reportertest.NewExpectDelivery()
	expectMetrics.AddTrigger(t, trigger(t1), wantRetryTags(t1))
	expectMetrics.AddTrigger(t, trigger(t2), wantRetryTags(t2))
	expectMetrics.AddTrigger(t, trigger(t3), wantRetryTags(t3))

	signal := make(chan struct{})
	syncPool, err := InitializeTestRetryPool(helper.Targets, retryPod, retryContainer, helper.PubsubClient)
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	p, err := GetFreePort()
	if err != nil {
		t.Fatalf("failed to get random free port: %v", err)
	}

	if _, err := StartSyncPool(ctx, syncPool, signal, time.Minute, p); err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}

	e1 := genTestEvent("foo1", "bar1", "id1", "source1")
	e2 := genTestEvent("foo2", "bar2", "id2", "source2")
	e3 := genTestEvent("foo3", "bar3", "id3", "source3")

	t.Run("target with same broker but different trigger did't receive retry events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		// Target1 for broker1 should receive the event e1.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), &e1)
			return nil
		})
		// Target2 for broker1 shouldn't receive the event e2.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), nil)
			return nil
		})

		// Only send an event to trigger topic 1.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, trigger(t1))
		expectMetrics.Verify(t)
	})

	t.Run("target with different broker did't receive retry events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		// Target1 for broker1 shouldn't  receive the event e3.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), nil)
			return nil
		})
		// Target2 for broker1 shouldn't receive the event e2.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), nil)
			return nil
		})
		// Target3 for broker2 should receive the event e3.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), &e3)
			return nil
		})

		// Only send an event to trigger topic 3.
		helper.SendEventToRetryQueue(ctx, t, t3.Key(), &e3)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, trigger(t3))
		expectMetrics.Verify(t)
	})

	t.Run("broker's target receive correct retry events", func(t *testing.T) {
		group, ctx := errgroup.WithContext(ctx)
		// Target1 for broker1 should receive the event e1.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), &e1)
			return nil
		})
		// Target2 for broker1 should receive the event e2.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), &e2)
			return nil
		})
		// Target3 for broker2 should receive the event e3.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), &e3)
			return nil
		})

		// Send different event to different trigger topic.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)
		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e2)
		helper.SendEventToRetryQueue(ctx, t, t3.Key(), &e3)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, trigger(t1))
		expectMetrics.Expect200(t, trigger(t2))
		expectMetrics.Expect200(t, trigger(t3))
		expectMetrics.Verify(t)
	})

	t.Run("broker's target receive correct retry events with the latest filter", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// Target t1 and t2 original don't have filter attributes,
		// update t1 filter attributes, and keep t2 filter attributes nil.
		t1.FilterAttributes = map[string]string{"subject": "foo"}
		helper.Targets.MutateBroker(t1.Namespace, t1.Broker, func(bm config.BrokerMutation) {
			bm.UpsertTargets(t1)
		})

		group, ctx := errgroup.WithContext(ctx)
		// Target1 for broker1 shouldn't receive the event e1, as filter attributes is updated.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(vctx, t, t1.Key(), nil)
			return nil
		})
		//  Target1 for broker1 should receive the event e1.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e1)
			return nil
		})

		// Send the same event to different trigger topic.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)
		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e1)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, trigger(t2))
		expectMetrics.Verify(t)
	})

	t.Run("event delivered after target retry queue update", func(t *testing.T) {
		helper.RenewTarget(ctx, t, t2.Key())
		signal <- struct{}{}

		group, ctx := errgroup.WithContext(ctx)
		// t2 should continue to receive the event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), &e1)
			return nil
		})

		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e1)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, trigger(t2))
		expectMetrics.Verify(t)
	})
}

func assertRetryHandlers(t *testing.T, p *RetryPool, targets config.Targets) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	targets.RangeAllTargets(func(t *config.Target) bool {
		if t.State == config.State_READY {
			wantHandlers[t.Key()] = true
		}
		return true
	})

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}

func genTestEvent(subject, t, id, source string) event.Event {
	e := event.New()
	e.SetSubject(subject)
	e.SetType(t)
	e.SetID(id)
	e.SetSource(source)
	eventutil.UpdateRemainingHops(context.Background(), &e, 123)
	return e
}

func wantRetryTags(target *config.Target) map[string]string {
	return map[string]string{
		"filter_type":    "any",
		"pod_name":       retryPod,
		"container_name": retryContainer,
	}
}
