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

package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"knative.dev/pkg/metrics"

	"github.com/google/knative-gcp/pkg/broker/config"
)

type DeliveryMetricsKey int

const (
	startDeliveryProcessingTime DeliveryMetricsKey = iota
)

type DeliveryReporter struct {
	podName               PodName
	containerName         ContainerName
	dispatchTimeInMsecM   *stats.Float64Measure
	processingTimeInMsecM *stats.Float64Measure
}

func (r *DeliveryReporter) register() error {
	return metrics.RegisterResourceView(
		&view.View{
			Name:        "event_count",
			Description: "Number of events delivered to a Trigger subscriber",
			Measure:     r.dispatchTimeInMsecM,
			Aggregation: view.Count(),
			TagKeys: []tag.Key{
				NamespaceNameKey,
				BrokerNameKey,
				TriggerNameKey,
				TriggerFilterTypeKey,
				ResponseCodeKey,
				ResponseCodeClassKey,
				PodNameKey,
				ContainerNameKey,
			},
		},
		&view.View{
			Name:        r.dispatchTimeInMsecM.Name(),
			Description: r.dispatchTimeInMsecM.Description(),
			Measure:     r.dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys: []tag.Key{
				NamespaceNameKey,
				BrokerNameKey,
				TriggerNameKey,
				TriggerFilterTypeKey,
				ResponseCodeKey,
				ResponseCodeClassKey,
				PodNameKey,
				ContainerNameKey,
			},
		},
		&view.View{
			Name:        r.processingTimeInMsecM.Name(),
			Description: r.processingTimeInMsecM.Description(),
			Measure:     r.processingTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys: []tag.Key{
				NamespaceNameKey,
				BrokerNameKey,
				TriggerNameKey,
				TriggerFilterTypeKey,
				PodNameKey,
				ContainerNameKey,
			},
		},
	)
}

// NewDeliveryReporter creates a new DeliveryReporter.
func NewDeliveryReporter(podName PodName, containerName ContainerName) (*DeliveryReporter, error) {
	r := &DeliveryReporter{
		podName:       podName,
		containerName: containerName,
		// dispatchTimeInMsecM records the time spent dispatching an event to
		// a Trigger subscriber, in milliseconds.
		dispatchTimeInMsecM: stats.Float64(
			"event_dispatch_latencies",
			"The time spent dispatching an event to a Trigger subscriber",
			stats.UnitMilliseconds,
		),
		// processingTimeInMsecM records the time spent between arrival at the Broker
		// and the delivery to the Trigger subscriber.
		processingTimeInMsecM: stats.Float64(
			"event_processing_latencies",
			"The time spent processing an event before it is dispatched to a Trigger subscriber",
			stats.UnitMilliseconds,
		),
	}

	if err := r.register(); err != nil {
		return nil, fmt.Errorf("failed to register delivery stats: %w", err)
	}
	return r, nil
}

// ReportEventDispatchTime captures dispatch times.
func (r *DeliveryReporter) ReportEventDispatchTime(ctx context.Context, d time.Duration) {
	attachments := getSpanContextAttachments(ctx)
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, r.dispatchTimeInMsecM.M(float64(d/time.Millisecond)), stats.WithAttachments(attachments))
}

// StartEventProcessing records the start of event processing for delivery within the given context.
func StartEventProcessing(ctx context.Context) context.Context {
	return context.WithValue(ctx, startDeliveryProcessingTime, time.Now())
}

// FinishEventProcessing captures event processing times. Requires StartDelivery to have been
// called previously using ctx.
func (r *DeliveryReporter) FinishEventProcessing(ctx context.Context) error {
	return r.reportEventProcessingTime(ctx, time.Now())
}

// ReportEventProcessingTime captures event processing times. Requires StartDelivery to have been
// called previously using ctx.
func (r *DeliveryReporter) reportEventProcessingTime(ctx context.Context, end time.Time) error {
	start, err := getStartDeliveryProcessingTime(ctx)
	if err != nil {
		return err
	}
	attachments := getSpanContextAttachments(ctx)
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, r.processingTimeInMsecM.M(float64(end.Sub(start)/time.Millisecond)), stats.WithAttachments(attachments))
	return nil
}

func filterTypeValue(v string) string {
	if v != "" {
		return v
	}
	// the default value if the filter attributes are empty.
	return "any"
}

func (r *DeliveryReporter) AddTags(ctx context.Context) (context.Context, error) {
	return tag.New(ctx,
		tag.Insert(PodNameKey, string(r.podName)),
		tag.Insert(ContainerNameKey, string(r.containerName)),
	)
}

func AddRespStatusCodeTags(ctx context.Context, responseCode int) (context.Context, error) {
	return tag.New(ctx,
		tag.Insert(ResponseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(ResponseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	)
}

func AddTargetTags(ctx context.Context, target *config.Target) (context.Context, error) {
	return tag.New(ctx,
		tag.Insert(NamespaceNameKey, target.Namespace),
		tag.Insert(BrokerNameKey, target.Broker),
		tag.Insert(TriggerNameKey, target.Name),
		tag.Insert(TriggerFilterTypeKey, filterTypeValue(target.FilterAttributes["type"])),
	)
}

func getStartDeliveryProcessingTime(ctx context.Context) (time.Time, error) {
	v := ctx.Value(startDeliveryProcessingTime)
	if time, ok := v.(time.Time); ok {
		return time, nil
	}
	return time.Time{}, fmt.Errorf("missing or invalid start time: %v", v)
}

// getSpanContextAttachments gets the attachment for exemplar trace.
func getSpanContextAttachments(ctx context.Context) metricdata.Attachments {
	attachments := map[string]interface{}{}
	if span := trace.FromContext(ctx); span != nil {
		if spanCtx := span.SpanContext(); spanCtx.IsSampled() {
			attachments[metricdata.AttachmentKeySpanContext] = spanCtx
		}
	}
	return attachments
}
