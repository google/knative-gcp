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

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

// stats_exporter is adapted from knative.dev/eventing/pkg/broker/ingress/stats_reporter.go
// with the following changes:
// - Metric descriptions are updated to match GCP broker specifics.
// - Removed StatsReporter interface and directly use helper methods instead.

type IngressReportArgs struct {
	Namespace    string
	Broker       string
	EventType    string
	ResponseCode int
}

func (r *IngressReporter) register() error {
	tagKeys := []tag.Key{
		NamespaceNameKey,
		BrokerNameKey,
		EventTypeKey,
		ResponseCodeKey,
		ResponseCodeClassKey,
		PodNameKey,
		ContainerNameKey,
	}

	// Create view to see our measurements.
	return metrics.RegisterResourceView(
		&view.View{
			Name:        r.eventCountM.Name(),
			Description: r.eventCountM.Description(),
			Measure:     r.eventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
	)
}

// NewIngressReporter creates a new StatsReporter.
func NewIngressReporter(podName PodName, containerName ContainerName) (*IngressReporter, error) {
	r := &IngressReporter{
		podName:       podName,
		containerName: containerName,
		eventCountM: stats.Int64(
			"event_count",
			"Number of events received by a Broker",
			stats.UnitDimensionless,
		),
	}
	if err := r.register(); err != nil {
		return nil, fmt.Errorf("failed to register ingress stats: %w", err)
	}
	return r, nil
}

// StatsReporter reports ingress metrics.
type IngressReporter struct {
	podName       PodName
	containerName ContainerName
	eventCountM   *stats.Int64Measure
}

func (r *IngressReporter) ReportEventCount(ctx context.Context, args IngressReportArgs) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(PodNameKey, string(r.podName)),
		tag.Insert(ContainerNameKey, string(r.containerName)),
		tag.Insert(NamespaceNameKey, args.Namespace),
		tag.Insert(BrokerNameKey, args.Broker),
		tag.Insert(EventTypeKey, EventTypeMetricValue(args.EventType)),
		tag.Insert(ResponseCodeKey, strconv.Itoa(args.ResponseCode)),
		tag.Insert(ResponseCodeClassKey, metrics.ResponseCodeClass(args.ResponseCode)),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	// Count does not support exemplar currently, but keeping this anyway for future support.
	attachments := getSpanContextAttachments(ctx)
	metrics.Record(tag, r.eventCountM.M(1), stats.WithAttachments(attachments))
	return nil
}
