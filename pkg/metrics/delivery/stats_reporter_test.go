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
	"testing"
	"time"

	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

func TestReportEventDispatchTime(t *testing.T) {
	resetMetrics()

	args := ReportArgs{
		namespace:  "testns",
		broker:     "testbroker",
		trigger:    "testtrigger",
		filterType: "testeventtype",
	}

	wantTags := map[string]string{
		metricskey.LabelNamespaceName:     "testns",
		metricskey.LabelBrokerName:        "testbroker",
		metricskey.LabelTriggerName:       "testtrigger",
		metricskey.LabelFilterType:        "testeventtype",
		metricskey.LabelResponseCode:      "202",
		metricskey.LabelResponseCodeClass: "2xx",
		metricskey.PodName:                "testpod",
		metricskey.ContainerName:          "testcontainer",
	}

	r := NewStatsReporter("testpod", "testcontainer")

	expectSuccess(t, func() error {
		return r.ReportEventDispatchTime(context.Background(), args, 1100*time.Millisecond, 202)
	})
	expectSuccess(t, func() error {
		return r.ReportEventDispatchTime(context.Background(), args, 9100*time.Millisecond, 202)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
	metricstest.CheckDistributionData(t, "event_dispatch_latencies", wantTags, 2, 1100.0, 9100.0)
}

func TestReportEventProcessingTime(t *testing.T) {
	resetMetrics()

	args := ReportArgs{
		namespace:  "testns",
		broker:     "testbroker",
		trigger:    "testtrigger",
		filterType: "testeventtype",
	}

	wantTags := map[string]string{
		metricskey.LabelNamespaceName: "testns",
		metricskey.LabelBrokerName:    "testbroker",
		metricskey.LabelTriggerName:   "testtrigger",
		metricskey.LabelFilterType:    "testeventtype",
		metricskey.PodName:            "testpod",
		metricskey.ContainerName:      "testcontainer",
	}

	r := NewStatsReporter("testpod", "testcontainer")

	// test ReportDispatchTime
	expectSuccess(t, func() error {
		return r.ReportEventProcessingTime(context.Background(), args, 1100*time.Millisecond)
	})
	expectSuccess(t, func() error {
		return r.ReportEventProcessingTime(context.Background(), args, 9100*time.Millisecond)
	})
	metricstest.CheckDistributionData(t, "event_processing_latencies", wantTags, 2, 1100.0, 9100.0)
}

func TestMetricsWithEmptySourceAndTypeFilter(t *testing.T) {
	resetMetrics()

	args := ReportArgs{
		namespace:  "testns",
		broker:     "testbroker",
		trigger:    "testtrigger",
		filterType: "", // No Filter Type
	}

	wantTags := map[string]string{
		metricskey.LabelNamespaceName:     "testns",
		metricskey.LabelBrokerName:        "testbroker",
		metricskey.LabelTriggerName:       "testtrigger",
		metricskey.LabelFilterType:        "any", // Expects this to be "any" instead of empty string
		metricskey.LabelResponseCode:      "202",
		metricskey.LabelResponseCodeClass: "2xx",
		metricskey.PodName:                "testpod",
		metricskey.ContainerName:          "testcontainer",
	}

	r := NewStatsReporter("testpod", "testcontainer")

	expectSuccess(t, func() error {
		return r.ReportEventDispatchTime(context.Background(), args, 1100*time.Millisecond, 202)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 1)
}

func resetMetrics() {
	// OpenCensus metrics carry global state that need to be reset between unit tests.
	metricstest.Unregister(
		"event_count",
		"event_dispatch_latencies",
		"event_processing_latencies")
	register()
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Errorf("Reporter expected success but got error: %v", err)
	}
}
