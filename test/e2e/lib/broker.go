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

	// If we reach this point, the projectID should have been set.
	projectID := os.Getenv(ProwProjectKey)
	f := map[string]interface{}{
		"metric.type":                      BrokerEventCountMetricType,
		"resource.type":                    BrokerMetricResourceType,
		"metric.label.event_type":          "e2e-testing-dummy",
		"resource.label.namespace_name":    client.Namespace,
		"metric.label.response_code":       http.StatusAccepted,
		"metric.label.response_code_class": pkgmetrics.ResponseCodeClass(http.StatusAccepted),
	}

	filter := metrics.StringifyStackDriverFilter(f)
	client.T.Logf("Filter expression: %s", filter)

	actualCount, err := client.StackDriverEventCountMetricFor(client.Namespace, projectID, filter)
	if err != nil {
		client.T.Fatalf("failed to get stackdriver event count metric: %v", err)
	}
	expectedCount := int64(1)
	if *actualCount != expectedCount {
		client.T.Errorf("Actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
		client.T.Fail()
	}
}
