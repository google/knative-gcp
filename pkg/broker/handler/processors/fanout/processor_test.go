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
	"strconv"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

func TestInvalidContext(t *testing.T) {
	p := &Processor{}
	e := event.New()
	err := p.Process(context.Background(), &e)
	if err != handlerctx.ErrBrokerKeyNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrBrokerKeyNotPresent)
	}
}

func TestFanoutSuccess(t *testing.T) {
	ch := make(chan *event.Event, 4)
	ns, broker := "ns", "broker"
	bk := config.BrokerKey(ns, broker)
	wantNum := 4
	wantTargets := newTestTargets(ns, broker, wantNum)
	gotTargets := memory.NewEmptyTargets()

	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		InterceptFunc: func(ctx context.Context, e *event.Event) *event.Event {
			b, _ := handlerctx.GetBroker(ctx)
			t, _ := handlerctx.GetTarget(ctx)
			gotTargets.MutateBroker(b.Namespace, b.Name, func(bm config.BrokerMutation) {
				bm.UpsertTargets(t)
			})
			return e
		},
	}

	p := &Processor{MaxConcurrency: 2, Targets: wantTargets}
	p.WithNext(next)

	e := event.New()
	e.SetID("id")
	e.SetSource("source")
	e.SetSubject("subject")
	e.SetType("type")

	defer func() {
		gotNum := 0
		for gotEvent := range ch {
			if diff := cmp.Diff(&e, gotEvent); diff != "" {
				t.Errorf("processed event (-want,+got): %v", diff)
			}
			gotNum++
		}
		if gotNum != wantNum {
			t.Errorf("fanout target number got=%d, want=%d", gotNum, wantNum)
		}
	}()

	ctx := handlerctx.WithBrokerKey(context.Background(), bk)
	if err := p.Process(ctx, &e); err != nil {
		t.Errorf("unexpected error from processing: %v", err)
	}
	// Close the channel so that the defer func will finish.
	close(ch)

	// Make sure the processor sets the broker and targets in the context.
	if !gotTargets.EqualsString(wantTargets.String()) {
		t.Errorf("targets received in the context got=%s, want=%s", gotTargets.String(), wantTargets.String())
	}
}

func TestFanoutPartialFailure(t *testing.T) {
	ch := make(chan *event.Event, 4)
	ns, broker := "ns", "broker"
	bk := config.BrokerKey(ns, broker)
	wantNum := 4
	wantTargets := newTestTargets(ns, broker, wantNum)

	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		OneTimeErr:   true,
	}

	p := &Processor{MaxConcurrency: 2, Targets: wantTargets}
	p.WithNext(next)

	e := event.New()
	e.SetID("id")
	e.SetSource("source")
	e.SetSubject("subject")
	e.SetType("type")

	defer func() {
		gotNum := 0
		for gotEvent := range ch {
			if diff := cmp.Diff(&e, gotEvent); diff != "" {
				t.Errorf("processed event (-want,+got): %v", diff)
			}
			gotNum++
		}
		if gotNum != wantNum {
			t.Errorf("fanout target number got=%d, want=%d", gotNum, wantNum)
		}
	}()

	ctx := handlerctx.WithBrokerKey(context.Background(), bk)
	if err := p.Process(ctx, &e); err == nil {
		t.Error("expect error from processing")
	}
	// Close the channel so that the defer func will finish.
	close(ch)
}

func newTestTargets(ns, broker string, num int) config.ReadonlyTargets {
	targets := memory.NewEmptyTargets()
	targets.MutateBroker(ns, broker, func(bm config.BrokerMutation) {
		for i := 0; i < num; i++ {
			bm.UpsertTargets(&config.Target{
				Name: fmt.Sprintf("target-%d", i),
				Id:   fmt.Sprintf("target-%d", i),
				FilterAttributes: map[string]string{
					"target": strconv.Itoa(i),
				},
			})
		}
	})
	return targets
}
