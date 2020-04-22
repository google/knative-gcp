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
	"net/http"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
	pooltesting "github.com/google/knative-gcp/pkg/broker/handler/pool/testing"
)

func TestWatchAndSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testProject := "test-project"
	helper, err := pooltesting.NewHelper(ctx, testProject)
	if err != nil {
		t.Fatalf("failed to create pool testing helper: %v", err)
	}
	defer helper.Close()

	signal := make(chan struct{})
	syncPool, err := NewSyncPool(ctx, helper.Targets,
		pool.WithPubsubClient(helper.PubsubClient),
		pool.WithProjectID(testProject))
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	t.Run("start sync pool creates no handler", func(t *testing.T) {
		_, err = pool.StartSyncPool(ctx, syncPool, signal)
		if err != nil {
			t.Errorf("unexpected error from starting sync pool: %v", err)
		}
		assertHandlers(t, syncPool, helper.Targets)
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
		assertHandlers(t, syncPool, helper.Targets)
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
		assertHandlers(t, syncPool, helper.Targets)
	})

	t.Run("deleting all brokers deletes all handlers", func(t *testing.T) {
		// clean up all brokers
		for _, b := range bs {
			helper.DeleteBroker(ctx, t, b.Key())
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertHandlers(t, syncPool, helper.Targets)
	})
}

func TestFanoutSyncPoolE2E(t *testing.T) {
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
	t2 := helper.GenerateTarget(ctx, t, b1.Key(), map[string]string{"subject": "foo"})
	t3 := helper.GenerateTarget(ctx, t, b2.Key(), nil)

	signal := make(chan struct{})
	syncPool, err := NewSyncPool(ctx, helper.Targets,
		pool.WithPubsubClient(helper.PubsubClient),
		pool.WithProjectID(testProject))
	if err != nil {
		t.Errorf("unexpected error from getting sync pool: %v", err)
	}

	if _, err := pool.StartSyncPool(ctx, syncPool, signal); err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}

	e := event.New()
	e.SetSubject("foo")
	e.SetType("type")
	e.SetID("id")
	e.SetSource("source")

	t.Run("broker's targets receive fanout events", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), &e)
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e)
		go helper.VerifyNextTargetEvent(vctx, t, t3.Key(), nil)

		// Only send an event to broker1.
		helper.SendEventToDecoupleQueue(ctx, t, b1.Key(), &e)
		<-vctx.Done()
	})

	t.Run("target with unmatching filter didn't receive event", func(t *testing.T) {
		t4 := helper.GenerateTarget(ctx, t, b1.Key(), map[string]string{"subject": "bar"})
		signal <- struct{}{}

		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// The old targets for broker1 should still receive the event.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), &e)
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), &e)
		// The new target for broker1 shouldn't receive the event
		// because the event doesn't match its filter.
		go helper.VerifyNextTargetEvent(vctx, t, t4.Key(), nil)
		// Target for broker2 still shouldn't receive any event.
		go helper.VerifyNextTargetEvent(vctx, t, t3.Key(), nil)

		// Only send an event to broker1.
		helper.SendEventToDecoupleQueue(ctx, t, b1.Key(), &e)
		<-vctx.Done()
	})

	t.Run("event sent to a broker didn't reach another broker's targets", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		// This time targets for broker1 shouldn't receive any event.
		go helper.VerifyNextTargetEvent(vctx, t, t1.Key(), nil)
		go helper.VerifyNextTargetEvent(vctx, t, t2.Key(), nil)
		// Target for broker2 should receive the event.
		go helper.VerifyNextTargetEvent(vctx, t, t3.Key(), &e)

		// Only send an event to broker2.
		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)
		<-vctx.Done()
	})

	t.Run("event failed initial delivery was sent to retry queue", func(t *testing.T) {
		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		go helper.VerifyAndRespondNextTargetEvent(ctx, t, t3.Key(), &e, nil, http.StatusInternalServerError)
		go helper.VerifyNextTargetRetryEvent(ctx, t, t3.Key(), &e)

		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)
		<-vctx.Done()
	})

	t.Run("event replied was sent to broker ingress", func(t *testing.T) {
		reply := event.New()
		reply.SetSubject("bar")
		reply.SetType("type")
		reply.SetID("id")
		reply.SetSource("source")

		// Set timeout context so that verification can be done before
		// exiting test func.
		vctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		go helper.VerifyAndRespondNextTargetEvent(ctx, t, t3.Key(), &e, &reply, http.StatusOK)
		go helper.VerifyNextBrokerIngressEvent(ctx, t, b2.Key(), &reply)

		helper.SendEventToDecoupleQueue(ctx, t, b2.Key(), &e)
		<-vctx.Done()
	})
}

func assertHandlers(t *testing.T, p *SyncPool, targets config.Targets) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	targets.RangeBrokers(func(b *config.Broker) bool {
		wantHandlers[b.Key()] = true
		return true
	})

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}
