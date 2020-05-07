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

package pool

import (
	"context"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	pooltesting "github.com/google/knative-gcp/pkg/broker/handler/pool/testing"
)

func TestRetryWatchAndSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testProject := "test-project"
	helper, err := pooltesting.NewHelper(ctx, testProject)
	if err != nil {
		t.Fatalf("failed to create pool testing helper: %v", err)
	}
	defer helper.Close()

	signal := make(chan struct{})
	syncPool, err := InitializeTestRetryPool(helper.Targets, helper.PubsubClient)
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	t.Run("start sync pool creates no handler", func(t *testing.T) {
		_, err = StartSyncPool(ctx, syncPool, signal)
		if err != nil {
			t.Errorf("unexpected error from starting sync pool: %v", err)
		}
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testProject := "test-project"

	helper, err := pooltesting.NewHelper(ctx, testProject)
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

	signal := make(chan struct{})
	syncPool, err := InitializeTestRetryPool(helper.Targets, helper.PubsubClient)
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	if _, err := StartSyncPool(ctx, syncPool, signal); err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}

	e1 := genTestEvent("foo1", "bar1", "id1", "source1")
	e2 := genTestEvent("foo2", "bar2", "id2", "source2")
	e3 := genTestEvent("foo3", "bar3", "id3", "source3")

	t.Run("target with same broker but different trigger did't receive retry events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// Target1 for broker1 should receive the event e1.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), &e1)
		// Target2 for broker1 should't receive the event e2.
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), nil)

		// Only send an event to trigger topic 1.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)
		<-vctx.Done()
	})

	t.Run("target with different broker did't receive retry events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// Target1 for broker1 should't  receive the event e3.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), nil)
		// Target2 for broker1 should't receive the event e2.
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), nil)
		// Target3 for broker2 should receive the event e3.
		go helper.VerifyNextTargetEvent(vctx, t, t3.Key(), &e3)

		// Only send an event to trigger topic 3.
		helper.SendEventToRetryQueue(ctx, t, t3.Key(), &e3)
		<-vctx.Done()
	})

	t.Run("broker's target receive correct retry events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// Target1 for broker1 should receive the event e1.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), &e1)
		// Target2 for broker1 should receive the event e2.
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e2)
		// Target3 for broker2 should receive the event e3.
		go helper.VerifyNextTargetEvent(vctx, t, t3.Key(), &e3)

		// Send different event to different trigger topic.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)
		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e2)
		helper.SendEventToRetryQueue(ctx, t, t3.Key(), &e3)
		<-vctx.Done()
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

		// Target1 for broker1 shouldn't receive the event e1, as filter attributes is updated.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), nil)
		//  Target1 for broker1 should receive the event e1.
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e1)

		// Send the same event to different trigger topic.
		helper.SendEventToRetryQueue(ctx, t, t1.Key(), &e1)
		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e1)
		<-vctx.Done()
	})

	t.Run("event delivered after target retry queue update", func(t *testing.T) {
		helper.RenewTarget(ctx, t, t2.Key())
		signal <- struct{}{}

		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// t2 should continue to receive the event.
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e1)

		helper.SendEventToRetryQueue(ctx, t, t2.Key(), &e1)
		<-vctx.Done()
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
		wantHandlers[t.Key()] = true
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
	return e
}
