/*
Copyright 2020 Google LLC.

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

	"github.com/google/knative-gcp/pkg/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
)

const (
	LoggingTraceKey        = "logging.googleapis.com/trace"
	LoggingSpanIDKey       = "logging.googleapis.com/spanId"
	LoggingTraceSampledKey = "logging.googleapis.com/trace_sampled"
)

func WithLogging(ctx context.Context, span *trace.Span) context.Context {
	if span == nil {
		return ctx
	}
	sc := span.SpanContext()
	return logging.With(ctx,
		zap.Stringer(LoggingTraceKey, sc.TraceID),
		zap.Stringer(LoggingSpanIDKey, sc.SpanID),
		zap.Bool(LoggingTraceSampledKey, sc.IsSampled()),
	)
}
