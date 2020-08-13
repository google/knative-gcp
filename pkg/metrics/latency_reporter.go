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
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

const LatencyMetricName string = "process_latencies"

type LatencyReporter struct {
	durationInMsecM *stats.Float64Measure
}

type ProcessType string

const (
	ResourceUpdateToBrokerCellNotified ProcessType = "ResourceUpdateToBrokerCellNotified"
)

func (r *LatencyReporter) register() error {
	return metrics.RegisterResourceView(
		&view.View{
			Name:        r.durationInMsecM.Name(),
			Description: r.durationInMsecM.Description(),
			Measure:     r.durationInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 100000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000, 20000, 50000, 1000000
			TagKeys: []tag.Key{
				NamespaceNameKey,
				ProcessTypeKey,
				EntityNameKey,
			},
		},
	)
}

// NewLatencyReporter creates a new LatencyReporter
func NewLatencyReporter() (*LatencyReporter, error) {
	r := &LatencyReporter{
		durationInMsecM: stats.Float64(
			LatencyMetricName,
			"Latency of a process in milliseconds",
			stats.UnitMilliseconds,
		),
	}
	if err := r.register(); err != nil {
		return nil, fmt.Errorf("failed to register the latency reporter: %w", err)
	}
	return r, nil
}

// ReportLatency records the value to the latency metric
func (r *LatencyReporter) ReportLatency(ctx context.Context, duration time.Duration, processType ProcessType, entityName string) error {
	tag, err := tag.New(
		ctx,
		tag.Insert(ProcessTypeKey, string(processType)),
		tag.Insert(EntityNameKey, entityName),
	)
	if err != nil {
		return fmt.Errorf("failed to create metrics tag: %v", err)
	}
	durationMetricValue := r.durationInMsecM.M(float64(duration / time.Millisecond))
	metrics.Record(tag, durationMetricValue)
	return nil
}