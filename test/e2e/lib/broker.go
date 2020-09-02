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
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

type BrokerMetricAssertion struct {
	ProjectID       string
	BrokerName      string
	BrokerNamespace string
	StartTime       time.Time
	CountPerType    map[string]int64
}

func (a BrokerMetricAssertion) Assert(client *monitoring.MetricClient) error {
	ctx := context.Background()
	start, err := ptypes.TimestampProto(a.StartTime)
	if err != nil {
		return err
	}
	end, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	it := client.ListTimeSeries(ctx, &monitoringpb.ListTimeSeriesRequest{
		Name:     fmt.Sprintf("projects/%s", a.ProjectID),
		Filter:   a.StackdriverFilter(),
		Interval: &monitoringpb.TimeInterval{StartTime: start, EndTime: end},
		View:     monitoringpb.ListTimeSeriesRequest_FULL,
	})
	gotCount := make(map[string]int64)
	for {
		ts, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		labels := ts.GetMetric().GetLabels()
		eventType := labels["event_type"]
		code, err := strconv.Atoi(labels["response_code"])
		if err != nil {
			return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
		}

		// Ignore undesired response code in metric count comparison due to flakiness in broker.
		// The sender pod will retry StatusCode 404, 503 and 500.
		// StatusCode 500 is currently for reducing flakiness caused by Workload Identity credential sync up.
		// We would remove it after https://github.com/google/knative-gcp/issues/1058 lands, as 500 error may indicate bugs in our code.
		if code == http.StatusNotFound || code == http.StatusServiceUnavailable || code == http.StatusInternalServerError {
			continue
		}

		if code != http.StatusAccepted {
			return fmt.Errorf("metric has unexpected response code: %v", ts.GetMetric())
		}
		gotCount[eventType] = gotCount[eventType] + metrics.SumCumulative(ts)
	}
	if diff := cmp.Diff(a.CountPerType, gotCount); diff != "" {
		return fmt.Errorf("unexpected broker metric count (-want, +got) = %v", diff)
	}
	return nil
}

func (a BrokerMetricAssertion) StackdriverFilter() string {
	filter := map[string]interface{}{
		"metric.type":                   BrokerEventCountMetricType,
		"resource.type":                 BrokerMetricResourceType,
		"resource.label.namespace_name": a.BrokerNamespace,
		"resource.label.broker_name":    a.BrokerName,
	}
	return metrics.StringifyStackDriverFilter(filter)
}
