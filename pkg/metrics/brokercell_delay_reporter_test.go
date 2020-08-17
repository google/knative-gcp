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
	"testing"
	"time"

	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"

	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

func TestReportLatency(t *testing.T) {
	// Arrange
	reportertest.ResetBrokerCellMetrics()
	r, err := NewBrokerCellLatencyReporter()
	if err != nil {
		t.Fatal(err)
	}
	latencySamples := []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		1000 * time.Millisecond,
	}
	// Act
	for _, latencySample := range latencySamples {
		reportertest.ExpectMetrics(t, func() error {
			r.ReportLatency(context.Background(), latencySample, "Trigger", "Test trigger", "TestNamespace")
			return nil
		})
	}
	// Assert
	expectedTags := map[string]string{
		labelResourceKind:             "Trigger",
		labelResourceName:             "Test trigger",
		metricskey.LabelNamespaceName: "TestNamespace",
	}
	metricstest.CheckDistributionData(t, LatencyMetricName, expectedTags, 3, 10.0, 1000.0)
}
