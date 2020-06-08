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
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	pkgmetrics "knative.dev/pkg/metrics"
)

func AssertBrokerMetrics(client *Client) {
	client.T.Helper()
	timeout := time.After(4 * time.Minute)
	var err error
	for {
		err = tryAssertBrokerMetrics(client)
		if err == nil {
			return
		}
		select {
		case <-timeout:
			client.T.Errorf("timeout checking metrics: %v", err)
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func tryAssertBrokerMetrics(client *Client) error {
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

	actualCount, err := client.StackDriverEventCountMetricFor(client.Namespace, projectID, filter)
	if err != nil {
		return fmt.Errorf("failed to get stackdriver event count metric: %w", err)
	}
	expectedCount := int64(1)
	if *actualCount != expectedCount {
		return fmt.Errorf("actual count different than expected count, actual: %d, expected: %d", actualCount, expectedCount)
	}
	return nil
}
