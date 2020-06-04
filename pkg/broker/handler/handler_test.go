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
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cepubsub "github.com/cloudevents/sdk-go/v2/protocol/pubsub"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

func testPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pubsub.Client, func()) {
	t.Helper()
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial test pubsub connection: %v", err)
	}
	close := func() {
		srv.Close()
		conn.Close()
	}
	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("failed to create test pubsub client: %v", err)
	}
	return c, close
}

func TestHandler(t *testing.T) {
	ctx := context.Background()
	c, close := testPubsubClient(ctx, t, "test-project")
	defer close()

	topic, err := c.CreateTopic(ctx, "test-topic")
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	sub, err := c.CreateSubscription(ctx, "test-sub", pubsub.SubscriptionConfig{
		Topic: topic,
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	p, err := cepubsub.New(context.Background(),
		cepubsub.WithClient(c),
		cepubsub.WithProjectID("test-project"),
		cepubsub.WithTopicID("test-topic"),
	)
	if err != nil {
		t.Fatalf("failed to create cloudevents pubsub protocol: %v", err)
	}

	eventCh := make(chan *event.Event)
	processor := &processors.FakeProcessor{PrevEventsCh: eventCh}
	h := NewHandler(sub, processor, time.Second, RetryPolicy{})
	h.Start(ctx, func(err error) {})
	defer h.Stop()
	if !h.IsAlive() {
		t.Error("start handler didn't bring it alive")
	}

	testEvent := event.New()
	testEvent.SetID("id")
	testEvent.SetSource("source")
	testEvent.SetSubject("subject")
	testEvent.SetType("type")

	t.Run("handle event success", func(t *testing.T) {
		if err := p.Send(ctx, binding.ToMessage(&testEvent)); err != nil {
			t.Errorf("failed to seed event to pubsub: %v", err)
		}
		gotEvent := nextEventWithTimeout(eventCh)
		if diff := cmp.Diff(&testEvent, gotEvent); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
	})

	t.Run("retry event on processing failure", func(t *testing.T) {
		unlock := processor.Lock()
		processor.OneTimeErr = true
		unlock()
		if err := p.Send(ctx, binding.ToMessage(&testEvent)); err != nil {
			t.Errorf("failed to seed event to pubsub: %v", err)
		}
		// On failure, the handler should nack the pubsub message.
		// And we should expect two deliveries.
		for i := 0; i < 2; i++ {
			gotEvent := nextEventWithTimeout(eventCh)
			if diff := cmp.Diff(&testEvent, gotEvent); diff != "" {
				t.Errorf("processed event (-want,+got): %v", diff)
			}
		}
	})

	t.Run("timeout on event processing", func(t *testing.T) {
		unlock := processor.Lock()
		processor.BlockUntilCancel = true
		unlock()
		if err := p.Send(ctx, binding.ToMessage(&testEvent)); err != nil {
			t.Errorf("failed to seed event to pubsub: %v", err)
		}
		gotEvent := nextEventWithTimeout(eventCh)
		if diff := cmp.Diff(&testEvent, gotEvent); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
		unlock = processor.Lock()
		if !processor.WasCancelled {
			t.Error("processor was not cancelled on timeout")
		}
		unlock()
	})
}

type alwaysErrProc struct {
	processors.BaseProcessor
	desiredErrCount, currErrCount int
}

func (p *alwaysErrProc) Process(_ context.Context, _ *event.Event) error {
	if p.currErrCount < p.desiredErrCount {
		p.currErrCount++
		return errors.New("always error")
	}
	return nil
}

func TestRetryBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, close := testPubsubClient(ctx, t, "test-project")
	defer close()

	topic, err := c.CreateTopic(ctx, "test-topic")
	if err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}
	sub, err := c.CreateSubscription(ctx, "test-sub", pubsub.SubscriptionConfig{
		Topic: topic,
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	p, err := cepubsub.New(context.Background(),
		cepubsub.WithClient(c),
		cepubsub.WithProjectID("test-project"),
		cepubsub.WithTopicID("test-topic"),
	)
	if err != nil {
		t.Fatalf("failed to create cloudevents pubsub protocol: %v", err)
	}

	delays := []time.Duration{}
	desiredErrCount := 8
	processor := &alwaysErrProc{desiredErrCount: desiredErrCount}
	h := NewHandler(sub, processor, time.Second, RetryPolicy{MinBackoff: time.Millisecond, MaxBackoff: 16 * time.Millisecond})
	// Mock sleep func to collect nack backoffs.
	h.delayNack = func(d time.Duration) {
		delays = append(delays, d)
	}
	h.Start(ctx, func(err error) {})
	defer h.Stop()
	if !h.IsAlive() {
		t.Error("start handler didn't bring it alive")
	}

	testEvent := event.New()
	testEvent.SetID("id")
	testEvent.SetSource("source")
	testEvent.SetSubject("subject")
	testEvent.SetType("type")

	if err := p.Send(ctx, binding.ToMessage(&testEvent)); err != nil {
		t.Errorf("failed to seed event to pubsub: %v", err)
	}

	// Wait until all desired errors were returned.
	// Then stop the handler by cancel the context.
	time.Sleep(time.Second)
	cancel()

	if len(delays) != desiredErrCount {
		t.Errorf("retry count got=%d, want=%d", len(delays), desiredErrCount)
	}
	if delays[0] != time.Millisecond {
		t.Errorf("initial nack delay got=%v, want=%v", delays[0], time.Millisecond)
	}
	// We expect exponential backoff until MaxBackoff
	for i := 1; i < len(delays); i++ {
		wantDelay := 2 * delays[i-1]
		if wantDelay > 16*time.Millisecond {
			wantDelay = 16 * time.Millisecond
		}
		if delays[i] != wantDelay {
			t.Errorf("delays[%d] got=%v, want=%v", i, delays[i], wantDelay)
		}
	}
}

func nextEventWithTimeout(eventCh <-chan *event.Event) *event.Event {
	select {
	case <-time.After(30 * time.Second):
		return nil
	case got := <-eventCh:
		return got
	}
}
