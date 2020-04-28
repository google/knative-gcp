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
	"log"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
)

// stats_exporter is adapted from knative.dev/eventing/pkg/broker/ingress/stats_reporter.go
// with the following changes:
// - Metric descriptions are updated to match GCP broker specifics.
// - Removed StatsReporter interface and directly use helper methods instead.

var (
	// eventCountM is a counter which records the number of events received
	// by the Broker.
	eventCountM = stats.Int64(
		"event_count",
		"Number of events received by a Broker",
		stats.UnitDimensionless,
	)

	// dispatchTimeInMsecM records the time spent dispatching an event to
	// a Channel, in milliseconds.
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
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 500, 1000, 5000, 10000
			TagKeys:     tagKeys,
		},
	)
	if err != nil {
		log.Printf("failed to register opencensus views, %s", err)
	}
}

func reportEventCount(ctxWithTag context.Context) {
	metrics.Record(ctxWithTag, eventCountM.M(1))
}

func reportEventDispatchTime(ctxWithTag context.Context, d time.Duration) {
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctxWithTag, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))

}

// generateTag returns a context with metrics tag injected.
func generateTag(ctx context.Context, ns, broker, eventType string, responseCode int) (context.Context, error) {
	return tag.New(
		ctx,
		tag.Insert(namespaceKey, ns),
		tag.Insert(brokerKey, broker),
		tag.Insert(eventTypeKey, eventType),
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	)
}

// InitMetricTagOrDie injects common metric tags into the context.
func InitMetricTagOrDie(ctx context.Context, podName, containerName string) context.Context {
	ctx, err := tag.New(ctx,
		tag.Insert(podKey, podName),
		tag.Insert(containerKey, containerName),
	)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to create metric tag", zap.Error(err))
	}
	return ctx
}
