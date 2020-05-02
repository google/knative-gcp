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

package ingress

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
)

// stats_exporter is adapted from knative.dev/eventing/pkg/broker/ingress/stats_reporter.go
// with the following changes:
// - Metric descriptions are updated to match GCP broker specifics.
// - Removed StatsReporter interface and directly use helper methods instead.

var (
	// dispatchTimeInMsecM records the time spent dispatching an event to
	// a decouple queue, in milliseconds.
	dispatchTimeInMsecM = stats.Float64(
		"event_dispatch_latencies",
		"The time spent dispatching an event to the decouple topic",
		stats.UnitMilliseconds,
	)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	namespaceKey         = tag.MustNewKey(metricskey.LabelNamespaceName)
	brokerKey            = tag.MustNewKey(metricskey.LabelBrokerName)
	eventTypeKey         = tag.MustNewKey(metricskey.LabelEventType)
	responseCodeKey      = tag.MustNewKey(metricskey.LabelResponseCode)
	responseCodeClassKey = tag.MustNewKey(metricskey.LabelResponseCodeClass)
	podKey               = tag.MustNewKey(metricskey.PodName)
	containerKey         = tag.MustNewKey(metricskey.ContainerName)
)

type reportArgs struct {
	namespace    string
	broker       string
	eventType    string
	responseCode int
}

func init() {
	register()
}

func register() {
	tagKeys := []tag.Key{
		namespaceKey,
		brokerKey,
		eventTypeKey,
		responseCodeKey,
		responseCodeClassKey,
		podKey,
		containerKey,
	}

	// Create view to see our measurements.
	err := view.Register(
		&view.View{
			Name:        "event_count",
			Description: "Number of events received by a Broker",
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Name:        dispatchTimeInMsecM.Name(),
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 500, 1000, 5000, 10000
			TagKeys:     tagKeys,
		},
	)
	if err != nil {
		log.Fatalf("failed to register opencensus views, %s", err)
	}
}

// NewStatsReporter creates a new StatsReporter.
func NewStatsReporter(args Args) *StatsReporter {
	return &StatsReporter{
		podName:       args.PodName,
		containerName: args.ContainerName,
	}
}

// StatsReporter reports ingress metrics.
type StatsReporter struct {
	podName       string
	containerName string
}

func (r *StatsReporter) reportEventDispatchTime(ctx context.Context, args reportArgs, d time.Duration) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(podKey, r.podName),
		tag.Insert(containerKey, r.containerName),
		tag.Insert(namespaceKey, args.namespace),
		tag.Insert(brokerKey, args.broker),
		tag.Insert(eventTypeKey, args.eventType),
		tag.Insert(responseCodeKey, strconv.Itoa(args.responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(args.responseCode)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(tag, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}
