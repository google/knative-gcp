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

package processors

import (
	"context"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/go-cmp/cmp"
)

func TestFakeProcessorAlwaysError(t *testing.T) {
	ch := make(chan *event.Event, 1)
	event := event.New()
	p := &FakeProcessor{AlwaysErr: true, PrevEventsCh: ch}

	defer func() {
		gotEvent := <-ch
		if diff := cmp.Diff(&event, gotEvent); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
	}()

	if err := p.Process(context.Background(), &event); err == nil {
		t.Error("expected error when AlwaysErr is set to true")
	}
}

func TestFakeProcessorOneTimeError(t *testing.T) {
	ch := make(chan *event.Event, 2)
	event := event.New()
	p := &FakeProcessor{OneTimeErr: true, PrevEventsCh: ch}

	defer func() {
		for i := 0; i < 2; i++ {
			gotEvent := <-ch
			if diff := cmp.Diff(&event, gotEvent); diff != "" {
				t.Errorf("processed event (-want,+got): %v", diff)
			}
		}
	}()

	if err := p.Process(context.Background(), &event); err == nil {
		t.Error("expected error the first time when OneTimeErr is set to true")
	}

	if err := p.Process(context.Background(), &event); err != nil {
		t.Error("expected non-error the second time when OneTimeErr is set to true")
	}
}

func TestFakeProcessorBlock(t *testing.T) {
	ch := make(chan *event.Event, 1)
	event := event.New()
	p := &FakeProcessor{BlockUntilCancel: true, PrevEventsCh: ch}

	defer func() {
		gotEvent := <-ch
		if diff := cmp.Diff(&event, gotEvent); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := p.Process(ctx, &event); err != nil {
		t.Errorf("unexpected error from blocking: %v", err)
	}
	if !p.WasCancelled {
		t.Error("FakeProcessor.WasCancelled got=false, want=true")
	}
}

func TestFakeProcessorModifyEvent(t *testing.T) {
	ch1 := make(chan *event.Event, 1)
	ch2 := make(chan *event.Event, 1)
	origin := event.New()
	modified := origin.Clone()
	modified.SetID("id")
	p1 := &FakeProcessor{
		PrevEventsCh: ch1,
		InterceptFunc: func(_ context.Context, _ *event.Event) *event.Event {
			return &modified
		},
	}
	p1.WithNext(&FakeProcessor{PrevEventsCh: ch2})

	go func() {
		gotEvent1 := <-ch1
		if diff := cmp.Diff(&origin, gotEvent1); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
		gotEvent2 := <-ch2
		if diff := cmp.Diff(&modified, gotEvent2); diff != "" {
			t.Errorf("processed event (-want,+got): %v", diff)
		}
	}()

	if err := p1.Process(context.Background(), &origin); err != nil {
		t.Errorf("unexpected error from processing: %v", err)
	}
}
