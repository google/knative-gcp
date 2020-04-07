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

	"github.com/cloudevents/sdk-go/v2/event"
)

func TestChainProcessors(t *testing.T) {
	eventCh := make(chan *event.Event, 2)
	p1 := &FakeProcessor{
		PrevEventsCh: eventCh,
		InterceptFunc: func(_ context.Context, e *event.Event) *event.Event {
			n := e.Clone()
			n.SetID("p1")
			return &n
		},
	}
	p2 := &FakeProcessor{PrevEventsCh: eventCh}

	all := ChainProcessors(p1, p2)
	e := event.New()
	e.SetID("p0")

	defer func() {
		gotEvent1 := <-eventCh
		if gotEvent1.ID() != "p0" {
			t.Errorf("processor1 previous event id got=%s, want=p0", gotEvent1.ID())
		}
		gotEvent2 := <-eventCh
		if gotEvent2.ID() != "p1" {
			t.Errorf("processor2 previous value got=%s, want=p1", gotEvent2.ID())
		}
	}()

	if err := all.Process(context.Background(), &e); err != nil {
		t.Errorf("chained processors got unexpected error: %v", err)
	}
}
