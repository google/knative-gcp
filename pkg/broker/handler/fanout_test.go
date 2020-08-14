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
	fanoutPod       = "fanout-pod"
	fanoutContainer = "fanout-container"
)

func TestFanoutWatchAndSync(t *testing.T) {
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
	syncPool, err := InitializeTestFanoutPool(ctx, fanoutPod, fanoutContainer, helper.Targets, helper.PubsubClient)
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
		assertFanoutHandlers(t, syncPool, helper.Targets)
	})

	t.Run("no handler created for not-ready broker", func(t *testing.T) {
		b := helper.GenerateBroker(ctx, t, "ns")
		helper.Targets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
			bm.SetState(config.State_UNKNOWN)
		})
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertFanoutHandlers(t, syncPool, helper.Targets)
	})

	bs := make([]*config.Broker, 0, 4)

	t.Run("adding new brokers creates new handlers", func(t *testing.T) {
		// First add some brokers.
		for i := 0; i < 4; i++ {
			bs = append(bs, helper.GenerateBroker(ctx, t, "ns"))
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertFanoutHandlers(t, syncPool, helper.Targets)
	})

	t.Run("adding and deleting brokers changes handlers", func(t *testing.T) {
		// Delete old and add new.
		for i := 0; i < 2; i++ {
			helper.DeleteBroker(ctx, t, bs[i].Key())
			bs = append(bs, helper.GenerateBroker(ctx, t, "ns"))
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertFanoutHandlers(t, syncPool, helper.Targets)
	})

	t.Run("deleting all brokers deletes all handlers", func(t *testing.T) {
		// clean up all brokers
		for _, b := range bs {
			helper.DeleteBroker(ctx, t, b.Key())
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertFanoutHandlers(t, syncPool, helper.Targets)
	})
}

func TestFanoutSyncPoolE2E(t *testing.T) {
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
	t2 := helper.GenerateTarget(ctx, t, b1.Key(), map[string]string{"subject": "foo"})
	t3 := helper.GenerateTarget(ctx, t, b2.Key(), nil)
	expectMetrics := reportertest.NewExpectDelivery()
	expectMetrics.AddTrigger(t, t1.Name, wantTags(t1))
	expectMetrics.AddTrigger(t, t2.Name, wantTags(t2))
	expectMetrics.AddTrigger(t, t3.Name, wantTags(t3))

	signal := make(chan struct{})
	syncPool, err := InitializeTestFanoutPool(
		ctx, fanoutPod, fanoutContainer, helper.Targets, helper.PubsubClient,
		WithDeliveryTimeout(500*time.Millisecond),
	)
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

	var hops int32 = 123
	e := event.New()
	e.SetSubject("foo")
	e.SetType("type")
	e.SetID("id")
	e.SetSource("source")
	eventutil.UpdateRemainingHops(ctx, &e, hops)

	t.Run("broker's targets receive fanout events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), &e)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), &e)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), nil)
			return nil
		})

		// Only send an event to broker1.
		helper.SendEventToDecoupleQueue(ctx, t, b1.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, t1.Name)
		expectMetrics.Expect200(t, t2.Name)
		expectMetrics.Verify(t)
	})

	t.Run("target with unmatching filter didn't receive event", func(t *testing.T) {
		t4 := helper.GenerateTarget(ctx, t, b1.Key(), map[string]string{"subject": "bar"})
		signal <- struct{}{}

		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		// The old targets for broker1 should still receive the event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), &e)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), &e)
			return nil
		})
		// The new target for broker1 shouldn't receive the event
		// because the event doesn't match its filter.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t4.Key(), nil)
			return nil
		})
		// Target for broker2 still shouldn't receive any event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), nil)
			return nil
		})

		// Only send an event to broker1.
		helper.SendEventToDecoupleQueue(ctx, t, b1.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, t1.Name)
		expectMetrics.Expect200(t, t2.Name)
		expectMetrics.Verify(t)
	})

	t.Run("event sent to a broker didn't reach another broker's targets", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		// This time targets for broker1 shouldn't receive any event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t1.Key(), nil)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), nil)
			return nil
		})
		// Target for broker2 should receive the event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), &e)
			return nil
		})

		// Only send an event to broker2.
		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, t3.Name)
		expectMetrics.Verify(t)
	})

	t.Run("event failed initial delivery was sent to retry queue", func(t *testing.T) {
		group, ctx := errgroup.WithContext(ctx)
		group.Go(func() error {
			helper.VerifyAndRespondNextTargetEvent(ctx, t, t3.Key(), &e, nil, http.StatusInternalServerError, 0)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextTargetRetryEvent(ctx, t, t3.Key(), &e)
			return nil
		})

		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.ExpectProcessing(t, t3.Name)
		expectMetrics.ExpectDelivery(t, t3.Name, 500)
		expectMetrics.Verify(t)
	})

	t.Run("event with delivery timeout was sent to retry queue", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		group, ctx := errgroup.WithContext(ctx)
		// Verify the event to t1 was received.
		group.Go(func() error {
			helper.VerifyNextTargetEventAndDelayResp(ctx, t, t1.Key(), &e, time.Second)
			return nil
		})
		// Because of the delay, t1 delivery should timeout.
		// Thus the event should have been sent to the retry queue.
		group.Go(func() error {
			helper.VerifyNextTargetRetryEvent(ctx, t, t1.Key(), &e)
			return nil
		})
		// The same event should be received by t2.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t2.Key(), &e)
			return nil
		})
		// But t2 shouldn't receive any retry event because the initial delay
		// was successful.
		group.Go(func() error {
			helper.VerifyNextTargetRetryEvent(ctx, t, t2.Key(), nil)
			return nil
		})

		helper.SendEventToDecoupleQueue(ctx, t, b1.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.ExpectProcessing(t, t1.Name)
		expectMetrics.ExpectTimeout(t, t1.Name)
		expectMetrics.Expect200(t, t2.Name)
		expectMetrics.Verify(t)
	})

	t.Run("event replied was sent to broker ingress", func(t *testing.T) {
		reply := event.New()
		reply.SetSubject("bar")
		reply.SetType("type")
		reply.SetID("id")
		reply.SetSource("source")

		// The reply to broker ingress should include the original hops.
		wantReply := reply.Clone()
		// -1 because the delivery processor should decrement remaining hops.
		eventutil.UpdateRemainingHops(ctx, &wantReply, hops-1)

		group, ctx := errgroup.WithContext(ctx)
		group.Go(func() error {
			helper.VerifyAndRespondNextTargetEvent(ctx, t, t3.Key(), &e, &reply, http.StatusOK, 0)
			return nil
		})
		group.Go(func() error {
			helper.VerifyNextBrokerIngressEvent(ctx, t, b2.Key(), &wantReply)
			return nil
		})

		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, t3.Name)
		expectMetrics.Verify(t)
	})

	t.Run("event delivered after broker decouple queue update", func(t *testing.T) {
		helper.RenewBroker(ctx, t, b2.Key())
		signal <- struct{}{}

		group, ctx := errgroup.WithContext(ctx)
		// Target for broker2 should continue to receive the event.
		group.Go(func() error {
			helper.VerifyNextTargetEvent(ctx, t, t3.Key(), &e)
			return nil
		})

		// Only send an event to broker2.
		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)

		if err := group.Wait(); err != nil {
			t.Error(err)
		}

		expectMetrics.Expect200(t, t3.Name)
		expectMetrics.Verify(t)
	})
}

func assertFanoutHandlers(t *testing.T, p *FanoutPool, targets config.Targets) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	targets.RangeBrokers(func(b *config.Broker) bool {
		if b.State == config.State_READY {
			wantHandlers[b.Key()] = true
		}
		return true
	})

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}

func wantTags(target *config.Target) map[string]string {
	return map[string]string{
		"trigger_name":   target.Name,
		"broker_name":    target.Broker,
		"namespace_name": target.Namespace,
		"filter_type":    "any",
		"pod_name":       fanoutPod,
		"container_name": fanoutContainer,
	}
}
