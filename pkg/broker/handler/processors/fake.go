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
	"errors"
	"sync"

	"github.com/cloudevents/sdk-go/v2/event"
)

// FakeProcessor is a fake processor used for testing.
type FakeProcessor struct {
	BaseProcessor

	// AlwaysErr once set will always immediately return error.
	AlwaysErr bool

	// OneTimeErr once set will immediately return an error exactly once.
	OneTimeErr bool

	// BlockUntilCancel will block until context gets cancelled only
	// if no error return is required.
	BlockUntilCancel bool

	// PrevEventsCh records the events from the previous processor.
	PrevEventsCh chan *event.Event

	// InterceptFunc delegates the event modification.
	// If nil, the original event will be passed.
	InterceptFunc func(context.Context, *event.Event) *event.Event

	// WasCancelled records whether the processing was cancelled.
	WasCancelled bool

	mux  sync.Mutex
	once sync.Once
}

var _ Interface = (*FakeProcessor)(nil)

// Process processes the event.
func (p *FakeProcessor) Process(ctx context.Context, event *event.Event) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	p.PrevEventsCh <- event

	if p.AlwaysErr {
		return errors.New("always error")
	}

	if p.OneTimeErr {
		var err error
		p.once.Do(func() {
			err = errors.New("process error")
		})
		if err != nil {
			return err
		}
	}

	if p.BlockUntilCancel {
		<-ctx.Done()
		p.WasCancelled = true
		return nil
	}

	ne := event
	if p.InterceptFunc != nil {
		ne = p.InterceptFunc(ctx, event)
	}
	return p.Next().Process(ctx, ne)
}

// Lock locks the FakeProcessor for any
// shared state change in different goroutines.
func (p *FakeProcessor) Lock() func() {
	p.mux.Lock()
	return p.mux.Unlock
}
