/*
Copyright 2019 Google LLC

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
	"strings"
	"testing"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/api/iterator"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/proto"
)

// TODO upstream to knative/pkg
// ListTimeSeries calls metricClient.ListTimeSeries and converts the returned Iterator to a list.
func ListTimeSeries(ctx context.Context, metricClient *monitoring.MetricClient, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	it := metricClient.ListTimeSeries(ctx, req)
	next, err := it.Next()
	var res []*monitoringpb.TimeSeries
	for ; err == nil; next, err = it.Next() {
		res = append(res, next)
	}
	if err != nil && err != iterator.Done {
		return res, err
	}
	return res, nil
}

// ListMetricDescriptors calls metricClient.ListMetricDescriptor and converts the returned
// Iterator to a list.
func ListMetricDescriptors(ctx context.Context, metricClient *monitoring.MetricClient, request *monitoringpb.ListMetricDescriptorsRequest) ([]*metricpb.MetricDescriptor, error) {
	it := metricClient.ListMetricDescriptors(ctx, request)
	next, err := it.Next()
	var res []*metricpb.MetricDescriptor
	for ; err == nil; next, err = it.Next() {
		res = append(res, next)
	}
	if err != nil && err != iterator.Done {
		return res, err
	}
	return res, nil
}

func StringifyStackDriverFilter(filter map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range filter {
		sb.WriteString(fmt.Sprintf("%s=\"%v\" ", k, v))
	}
	return strings.TrimSuffix(sb.String(), " ")
}

type Assertion interface {
	Assert(*monitoring.MetricClient) error
}

func CheckAssertions(t *testing.T, assertions ...Assertion) {
	t.Helper()
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		t.Error(err)
		return
	}
	timeout := time.After(8 * time.Minute)
	for {
		errors := make([]error, 0, len(assertions))
		for _, assertion := range assertions {
			if err := assertion.Assert(client); err != nil {
				errors = append(errors, err)
			}
		}
		if len(errors) == 0 {
			return
		}
		select {
		case <-timeout:
			t.Errorf("timeout checking metrics")
			for _, err := range errors {
				t.Error(err)
			}
			return
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func SumCumulative(ts *monitoringpb.TimeSeries) int64 {
	var startTime *timestamp.Timestamp
	var lastVal int64
	var sum int64
	for _, point := range ts.GetPoints() {
		if !proto.Equal(point.GetInterval().GetStartTime(), startTime) {
			lastVal = 0
		}
		val := point.GetValue().GetInt64Value()
		sum += val - lastVal
		lastVal = val
	}
	return sum
}

func SumDistCount(ts *monitoringpb.TimeSeries) int64 {
	var sum int64
	for _, point := range ts.GetPoints() {
		sum += point.GetValue().GetDistributionValue().GetCount()
	}
	return sum
}
