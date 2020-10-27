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

	"github.com/cloudevents/sdk-go/v2/event"
)

// Interface is the interface to process an event.
// If a processor can rely on its successor(s) for timeout,
// then it shouldn't explicitly handle timeout. If a processor has some
// internal long running processing, then it must handle timeout by itself.
type Interface interface {
	// Process processes an event. It may decide to terminate the processing early
	// or it can pass the event to the next Processor for further processing.
	Process(ctx context.Context, e *event.Event) error
	// Next returns the next Processor to process events.
	Next() Interface
}

// ChainableProcessor is the interface to chainable Processor.
type ChainableProcessor interface {
	Interface

	// WithNext sets the next Processor to pass the event.
	WithNext(ChainableProcessor) ChainableProcessor
}

// BaseProcessor provoides implementation to set and get
// next processor. It can gracefully handle the case where the next
// processor doesn't exist.
type BaseProcessor struct {
	n ChainableProcessor
}

// Next returns the next processor otherwise it will return a
// no-op processor so that caller doesn't need to worry about
// calling a nil processor.
func (p *BaseProcessor) Next() Interface {
	if p.n == nil {
		return noop
	}
	return p.n
}

// WithNext sets the next Processor to pass the event.
func (p *BaseProcessor) WithNext(n ChainableProcessor) ChainableProcessor {
	p.n = n
	return p.n
}

// ChainProcessors chains the given processors in order.
func ChainProcessors(first ChainableProcessor, rest ...ChainableProcessor) Interface {
	next := first
	for _, p := range rest {
		next = next.WithNext(p)
	}
	return first
}

var noop = &noOpProcessor{}

type noOpProcessor struct{}

func (p noOpProcessor) Process(_ context.Context, _ *event.Event) error {
	return nil
}

// Next here shouldn't really be called because
// reaching noop-processor already means the chain of processors reach a nil.
func (p noOpProcessor) Next() Interface {
	return nil
}
