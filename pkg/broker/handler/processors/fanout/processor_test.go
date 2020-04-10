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
	"go.uber.org/zap/zaptest"
	"knative.dev/eventing/pkg/logging"

	"github.com/google/knative-gcp/pkg/broker/config"
	handlerctx "github.com/google/knative-gcp/pkg/broker/handler/context"
	"github.com/google/knative-gcp/pkg/broker/handler/processors"
)

func TestInvalidContext(t *testing.T) {
	p := &Processor{}
	e := event.New()
	err := p.Process(context.Background(), &e)
	if err != handlerctx.ErrBrokerNotPresent {
		t.Errorf("Process error got=%v, want=%v", err, handlerctx.ErrBrokerNotPresent)
	}
}

func TestFanoutSuccess(t *testing.T) {
	ch := make(chan *event.Event, 4)
	ns, broker := "ns", "broker"
	wantNum := 4
	brokerConfig := newBrokerConfig(ns, broker, wantNum)
	targetCh := make(chan *config.Target, wantNum)

	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		InterceptFunc: func(ctx context.Context, e *event.Event) *event.Event {
			t, _ := handlerctx.GetTarget(ctx)
			select {
			case targetCh <- t:
				return e
			default:
				return nil
			}
		},
	}

	p := &Processor{MaxConcurrency: 2}
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

	ctx := handlerctx.WithBroker(context.Background(), brokerConfig)
	ctx = logging.WithLogger(ctx, zaptest.NewLogger(t))
	if err := p.Process(ctx, &e); err != nil {
		t.Errorf("unexpected error from processing: %v", err)
	}
	close(targetCh)
	// Close the channel so that the defer func will finish.
	close(ch)

	gotTargets := make(map[string]*config.Target)
	for target := range targetCh {
		gotTargets[target.Name] = target
	}
	// Make sure the processor sets the broker and targets in the context.
	if diff := cmp.Diff(brokerConfig.Targets, gotTargets); diff != "" {
		t.Errorf("unexpected (-want, +got) = %v", diff)
	}
}

func TestFanoutPartialFailure(t *testing.T) {
	ch := make(chan *event.Event, 4)
	ns, broker := "ns", "broker"
	wantNum := 4
	brokerConfig := newBrokerConfig(ns, broker, wantNum)

	next := &processors.FakeProcessor{
		PrevEventsCh: ch,
		OneTimeErr:   true,
	}

	p := &Processor{MaxConcurrency: 2}
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

	ctx := handlerctx.WithBroker(context.Background(), brokerConfig)
	if err := p.Process(ctx, &e); err == nil {
		t.Error("expect error from processing")
	}
	// Close the channel so that the defer func will finish.
	close(ch)
}

func newBrokerConfig(ns, broker string, num int) *config.Broker {
	targets := make(map[string]*config.Target)
	for i := 0; i < num; i++ {
		name := fmt.Sprintf("target-%d", i)
		targets[name] = &config.Target{
			Namespace: ns,
			Name:      name,
			Broker:    broker,
			Id:        fmt.Sprintf("target-%d", i),
			FilterAttributes: map[string]string{
				"target": strconv.Itoa(i),
			},
		}
	}
	return &config.Broker{
		Namespace: ns,
		Name:      broker,
		Targets:   targets,
	}
}
