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

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

type DeliveryReporter struct {
	podName               PodName
	containerName         ContainerName
	dispatchTimeInMsecM   *stats.Float64Measure
	processingTimeInMsecM *stats.Float64Measure
}

type DeliveryReportArgs struct {
	Namespace  string
	Broker     string
	Trigger    string
	FilterType string
}

func (r *DeliveryReporter) register() error {
	return view.Register(
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
func (r *DeliveryReporter) ReportEventDispatchTime(ctx context.Context, args DeliveryReportArgs, d time.Duration, responseCode int) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(NamespaceNameKey, args.Namespace),
		tag.Insert(BrokerNameKey, args.Broker),
		tag.Insert(TriggerNameKey, args.Trigger),
		tag.Insert(TriggerFilterTypeKey, filterTypeValue(args.FilterType)),
		tag.Insert(PodNameKey, string(r.podName)),
		tag.Insert(ContainerNameKey, string(r.containerName)),
		tag.Insert(ResponseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(ResponseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(tag, r.dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

// ReportEventProcessingTime captures event processing times.
func (r *DeliveryReporter) ReportEventProcessingTime(ctx context.Context, args DeliveryReportArgs, d time.Duration) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(NamespaceNameKey, args.Namespace),
		tag.Insert(BrokerNameKey, args.Broker),
		tag.Insert(TriggerNameKey, args.Trigger),
		tag.Insert(TriggerFilterTypeKey, filterTypeValue(args.FilterType)),
		tag.Insert(PodNameKey, string(r.podName)),
		tag.Insert(ContainerNameKey, string(r.containerName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(tag, r.processingTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func filterTypeValue(v string) string {
	if v != "" {
		return v
	}
	// the default value if the filter attributes are empty.
	return "any"
}
