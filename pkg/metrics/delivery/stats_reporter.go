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

package delivery

import (
	"context"
	"fmt"

	m "github.com/google/knative-gcp/pkg/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
	"log"
	"strconv"
	"time"
)

var (
	// dispatchTimeInMsecM records the time spent dispatching an event to
	// a Trigger subscriber, in milliseconds.
	dispatchTimeInMsecM = stats.Float64(
		"event_dispatch_latencies",
		"The time spent dispatching an event to a Trigger subscriber",
		stats.UnitMilliseconds,
	)

	// processingTimeInMsecM records the time spent between arrival at the Broker
	// and the delivery to the Trigger subscriber.
	processingTimeInMsecM = stats.Float64(
		"event_processing_latencies",
		"The time spent processing an event before it is dispatched to a Trigger subscriber",
		stats.UnitMilliseconds,
	)
)

type PodName string
type ContainerName string

type StatsReporter struct {
	podName       PodName
	containerName ContainerName
}

type ReportArgs struct {
	namespace  string
	broker     string
	trigger    string
	filterType string
}

func init() {
	register()
}

func register() {
	err := view.Register(
		&view.View{
			Name:        "event_count",
			Description: "Number of events received by a Trigger",
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Count(),
			TagKeys: []tag.Key{
				m.NamespaceNameKey,
				m.BrokerNameKey,
				m.TriggerNameKey,
				m.TriggerFilterTypeKey,
				m.ResponseCodeKey,
				m.ResponseCodeClassKey,
				m.PodNameKey,
				m.ContainerNameKey,
			},
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys: []tag.Key{
				m.NamespaceNameKey,
				m.BrokerNameKey,
				m.TriggerNameKey,
				m.TriggerFilterTypeKey,
				m.ResponseCodeKey,
				m.ResponseCodeClassKey,
				m.PodNameKey,
				m.ContainerNameKey,
			},
		},
		&view.View{
			Description: processingTimeInMsecM.Description(),
			Measure:     processingTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys: []tag.Key{
				m.NamespaceNameKey,
				m.BrokerNameKey,
				m.TriggerNameKey,
				m.TriggerFilterTypeKey,
				m.PodNameKey,
				m.ContainerNameKey,
			},
		},
	)

	if err != nil {
		log.Fatalf("failed to register opencensus views, %s", err)
	}
}

// NewStatsReporter creates a new StatsReporter.
func NewStatsReporter(podName PodName, containerName ContainerName) *StatsReporter {
	return &StatsReporter{
		podName:       podName,
		containerName: containerName,
	}
}

// ReportEventDispatchTime captures dispatch times.
func (r *StatsReporter) ReportEventDispatchTime(ctx context.Context, args ReportArgs, d time.Duration, responseCode int) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(m.NamespaceNameKey, args.namespace),
		tag.Insert(m.BrokerNameKey, args.broker),
		tag.Insert(m.TriggerNameKey, args.trigger),
		tag.Insert(m.TriggerFilterTypeKey, filterTypeValue(args.filterType)),
		tag.Insert(m.PodNameKey, string(r.podName)),
		tag.Insert(m.ContainerNameKey, string(r.containerName)),
		tag.Insert(m.ResponseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(m.ResponseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(tag, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

// ReportEventProcessingTime captures event processing times.
func (r *StatsReporter) ReportEventProcessingTime(ctx context.Context, args ReportArgs, d time.Duration) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(m.NamespaceNameKey, args.namespace),
		tag.Insert(m.BrokerNameKey, args.broker),
		tag.Insert(m.TriggerNameKey, args.trigger),
		tag.Insert(m.TriggerFilterTypeKey, filterTypeValue(args.filterType)),
		tag.Insert(m.PodNameKey, string(r.podName)),
		tag.Insert(m.ContainerNameKey, string(r.containerName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(tag, processingTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func filterTypeValue(v string) string {
	if v != "" {
		return v
	}
	// the default value if the filter attributes are empty.
	return "any"
}
