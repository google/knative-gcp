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

	_ "knative.dev/pkg/metrics/testing"

	reportertest "github.com/google/knative-gcp/pkg/metrics/testing"
	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
)

func TestStatsReporter(t *testing.T) {
	reportertest.ResetIngressMetrics()

	args := IngressReportArgs{
		EventType:    "google.cloud.scheduler.job.v1.executed",
		ResponseCode: 202,
	}
	wantTags := map[string]string{
		metricskey.LabelEventType:         "google.cloud.scheduler.job.v1.executed",
		metricskey.LabelResponseCode:      "202",
		metricskey.LabelResponseCodeClass: "2xx",
		metricskey.ContainerName:          "testcontainer",
		metricskey.PodName:                "testpod",
	}

	r, err := NewIngressReporter(PodName("testpod"), ContainerName("testcontainer"))
	if err != nil {
		t.Fatal(err)
	}

	// test ReportEventCount
	reportertest.ExpectMetrics(t, func() error {
		return r.ReportEventCount(context.Background(), args)
	})
	reportertest.ExpectMetrics(t, func() error {
		return r.ReportEventCount(context.Background(), args)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
}

func TestStatsReporterWithCustomEventType(t *testing.T) {
	reportertest.ResetIngressMetrics()

	args := IngressReportArgs{
		EventType:    "testeventtype",
		ResponseCode: 202,
	}
	wantTags := map[string]string{
		metricskey.LabelEventType:         "custom",
		metricskey.LabelResponseCode:      "202",
		metricskey.LabelResponseCodeClass: "2xx",
		metricskey.ContainerName:          "testcontainer",
		metricskey.PodName:                "testpod",
	}

	r, err := NewIngressReporter(PodName("testpod"), ContainerName("testcontainer"))
	if err != nil {
		t.Fatal(err)
	}

	// test ReportEventCount
	reportertest.ExpectMetrics(t, func() error {
		return r.ReportEventCount(context.Background(), args)
	})
	reportertest.ExpectMetrics(t, func() error {
		return r.ReportEventCount(context.Background(), args)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)
}
