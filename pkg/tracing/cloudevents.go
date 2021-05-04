/*
Copyright 2020 Google LLC.
Portions copyright 2020 CNCF

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

package tracing

import (
	"context"
	"strings"

	"github.com/cloudevents/sdk-go/v2/extensions"
	"github.com/lightstep/tracecontext.go/traceparent"
	"github.com/lightstep/tracecontext.go/tracestate"
	"go.opencensus.io/trace"
	octs "go.opencensus.io/trace/tracestate"
)

// FromSpanContext populates DistributedTracingExtension from a SpanContext.
// This is copied from cloudevents/sdk-go v2.3.1, removed in 2.4.0 by
// https://github.com/cloudevents/sdk-go/pull/634.
func FromSpanContext(sc trace.SpanContext) extensions.DistributedTracingExtension {
	tp := traceparent.TraceParent{
		TraceID: sc.TraceID,
		SpanID:  sc.SpanID,
		Flags: traceparent.Flags{
			Recorded: sc.IsSampled(),
		},
	}

	entries := make([]string, 0, len(sc.Tracestate.Entries()))
	for _, entry := range sc.Tracestate.Entries() {
		entries = append(entries, strings.Join([]string{entry.Key, entry.Value}, "="))
	}

	return extensions.DistributedTracingExtension{
		TraceParent: tp.String(),
		TraceState:  strings.Join(entries, ","),
	}
}

// ToSpanContext creates a SpanContext from a DistributedTracingExtension instance.
// This is copied from cloudevents/sdk-go v2.3.1, removed in 2.4.0 by
// https://github.com/cloudevents/sdk-go/pull/634.
func ToSpanContext(d extensions.DistributedTracingExtension) (trace.SpanContext, error) {
	tp, err := traceparent.ParseString(d.TraceParent)
	if err != nil {
		return trace.SpanContext{}, err
	}
	sc := trace.SpanContext{
		TraceID: tp.TraceID,
		SpanID:  tp.SpanID,
	}
	if tp.Flags.Recorded {
		sc.TraceOptions |= 1
	}

	if ts, err := tracestate.ParseString(d.TraceState); err == nil {
		entries := make([]octs.Entry, 0, len(ts))
		for _, member := range ts {
			var key string
			if member.Tenant != "" {
				// Due to github.com/lightstep/tracecontext.go/issues/6,
				// the meaning of Vendor and Tenant are swapped here.
				key = member.Vendor + "@" + member.Tenant
			} else {
				key = member.Vendor
			}
			entries = append(entries, octs.Entry{Key: key, Value: member.Value})
		}
		sc.Tracestate, _ = octs.New(nil, entries...)
	}

	return sc, nil
}

// StartChildSpan adds a child span to the trace parent in the given context.
// This is copied from cloudevents/sdk-go v2.3.1, removed in 2.4.0 by
// https://github.com/cloudevents/sdk-go/pull/634.
func StartChildSpan(ctx context.Context, d extensions.DistributedTracingExtension, name string, opts ...trace.StartOption) (context.Context, *trace.Span) {
	if sc, err := ToSpanContext(d); err == nil {
		tSpan := trace.FromContext(ctx)
		ctx, span := trace.StartSpanWithRemoteParent(ctx, name, sc, opts...)
		if tSpan != nil {
			// Add link to the previous in-process trace.
			tsc := tSpan.SpanContext()
			span.AddLink(trace.Link{
				TraceID: tsc.TraceID,
				SpanID:  tsc.SpanID,
				Type:    trace.LinkTypeParent,
			})
		}
		return ctx, span
	}
	return ctx, nil
}
