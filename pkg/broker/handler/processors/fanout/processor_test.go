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
	"sort"
	"strconv"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

var (
	// config.TargetKey is not inherently diffable, because it has unexported fields. In addition,
	// while we don't care about the order in which the keys are stored, the diff needs a consistent
	// ordering. So, to make this consistently diffable, we first change from config.TargetKey to
	// its string equivalent. Then we sort those strings. This will be done for all
	// []*config.TargetKey variables.
	diffTargetKeySlice = cmp.Transformer("SortedToString", func(in []*config.TargetKey) []string {
		out := make([]string, len(in))
		for i := range in {
			out[i] = in[i].String()
		}
		sort.Strings(out)
		return out
	})
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
	bk := config.TestOnlyBrokerKey(ns, broker)
	wantNum := 4
	testTargets := newTestTargets(bk, wantNum)
	wantTargets := make([]*config.TargetKey, 0, wantNum)
	testTargets.RangeAllTargets(func(t *config.Target) bool {
		wantTargets = append(wantTargets, t.Key())
		return true
	})
	var gotTargets []*config.TargetKey
	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		InterceptFunc: func(ctx context.Context, e *event.Event) *event.Event {
			t, _ := handlerctx.GetTargetKey(ctx)
			gotTargets = append(gotTargets, t)
			return e
		},
	}

	p := &Processor{MaxConcurrency: 2, Targets: testTargets}
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
	if diff := cmp.Diff(wantTargets, gotTargets, diffTargetKeySlice); diff != "" {
		t.Errorf("got target keys (-want,+got): %v", diff)
	}
}

func TestFanoutPartialFailure(t *testing.T) {
	ch := make(chan *event.Event, 4)
	ns, broker := "ns", "broker"
	bk := config.TestOnlyBrokerKey(ns, broker)
	wantNum := 4
	testTargets := newTestTargets(bk, wantNum)

	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		OneTimeErr:   true,
	}

	p := &Processor{MaxConcurrency: 2, Targets: testTargets}
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

func newTestTargets(key *config.CellTenantKey, num int) config.ReadonlyTargets {
	targets := memory.NewEmptyTargets()
	targets.MutateCellTenant(key, func(bm config.CellTenantMutation) {
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
