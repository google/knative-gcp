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

package lib

import (
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"net/http"
	"os"
	"time"

	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	pkgmetrics "knative.dev/pkg/metrics"
)

func AssertBrokerMetrics(client *Client) {
	client.T.Helper()
	sleepTime := 4 * time.Minute
	client.T.Logf("Sleeping %s to make sure metrics were pushed to stackdriver", sleepTime.String())
	time.Sleep(sleepTime)

	AssertBrokerEventCountMetric(client)
	AssertTriggerEventCountMetric(client)
}

func AssertBrokerEventCountMetric(client *Client) {
	// If we reach this point, the projectID should have been set.
	projectID := os.Getenv(ProwProjectKey)
	f := map[string]interface{}{
		"metric.type":                      BrokerEventCountMetricType,
		"resource.type":                    BrokerMetricResourceType,
		"metric.label.event_type":          E2EDummyEventType,
		"resource.label.namespace_name":    client.Namespace,
		"metric.label.response_code":       http.StatusAccepted,
		"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusAccepted),
	}

	filter := metrics.StringifyStackDriverFilter(f)
	client.T.Logf("Filter expression: %s", filter)
	timeseries, err := client.StackDriverTimeSeriesFor(projectID, filter)
	if err != nil {
		client.T.Fatalf("failed to get stackdriver timeseries: %v", err)
	}

	AssertMetricCount(client, timeseries, 1 /* expectedCount */)
}

func AssertTriggerEventCountMetric(client *Client) {
	// If we reach this point, the projectID should have been set.
	projectID := os.Getenv(ProwProjectKey)
	f := map[string]interface{}{
		"metric.type":                      TriggerEventCountMetricType,
		"resource.type":                    TriggerMetricResourceType,
		"metric.label.filter_type":         E2EDummyEventType,
		"resource.label.namespace_name":    client.Namespace,
		"metric.label.response_code":       http.StatusAccepted,
		"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusAccepted),
	}

	filter := metrics.StringifyStackDriverFilter(f)
	client.T.Logf("Filter expression: %s", filter)

	timeseries, err := client.StackDriverTimeSeriesFor(projectID, filter)
	if err != nil {
		client.T.Fatalf("failed to get stackdriver timeseries: %v", err)
	}

	AssertMetricCount(client, timeseries, 1 /* expectedCount */)
}
