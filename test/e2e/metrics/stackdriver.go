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
	"github.com/golang/protobuf/ptypes/duration"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/monitoring/apiv3"
	googlepb "github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	metricType    = "custom.googleapis.com/cloud.run/source/event_count"
	resourceType  = "global"
	resourceGroup = "storages.events.cloud.run"
	eventType     = "google.storage.object.finalize"
)

func NewStackDriverMetricClient() (*monitoring.MetricClient, error) {
	return monitoring.NewMetricClient(context.TODO())
}

// StackDriverListTimeSeriesRequestOption enables further configuration of a ListTimeSeriesRequest.
type StackDriverListTimeSeriesRequestOption func(*monitoringpb.ListTimeSeriesRequest)

func NewStackDriverListTimeSeriesRequest(projectID string, o ...StackDriverListTimeSeriesRequestOption) *monitoringpb.ListTimeSeriesRequest {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:        fmt.Sprintf("projects/%s", projectID),
		Aggregation: &monitoringpb.Aggregation{},
	}
	for _, opt := range o {
		opt(req)
	}
	return req
}

func WithStackDriverFilter(filter map[string]interface{}) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		var sb strings.Builder
		for k, v := range filter {
			sb.WriteString(fmt.Sprintf("%s=\"%v\" ", k, v))
		}
		r.Filter = strings.TrimSuffix(sb.String(), " ")
	}
}

func WithStackDriverInterval(startSecs, endSecs int64) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		r.Interval = &monitoringpb.TimeInterval{
			StartTime: &googlepb.Timestamp{Seconds: startSecs},
			EndTime:   &googlepb.Timestamp{Seconds: endSecs}}
	}
}

func WithStackDriverAlignmentPeriod(seconds int64) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		r.Aggregation.AlignmentPeriod = &duration.Duration{Seconds: seconds},
	}
}

func WithStackDriverPerSeriesAligner(aligner monitoringpb.Aggregation_Aligner) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		r.Aggregation.PerSeriesAligner = aligner
	}
}

func WithStackDriverCrossSeriesReducer(reducer monitoringpb.Aggregation_Reducer) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		r.Aggregation.CrossSeriesReducer = reducer
	}
}

func main() {
	ctx := context.Background()

	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	projectID := "knative-project-228222"
	filter := fmt.Sprintf("metric.type=\"%v\" resource.type=\"%v\" metric.label.resource_group=\"%v\" metric.label.event_type=\"%v\"",
		metricType,
		resourceType,
		resourceGroup,
		eventType)
	req := monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%v", projectID),
		Filter: filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: &googlepb.Timestamp{
				Seconds: time.Now().Add(-time.Minute * 2).Unix()},
			EndTime: &googlepb.Timestamp{
				Seconds: time.Now().Unix()}},
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod:    &duration.Duration{Seconds: 2 * 60},
			PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_DELTA,
			CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_SUM,
		}}
	it := client.ListTimeSeries(ctx, &req)

	for {
		timeseries, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Failed to iterate over timeseries: %v", err)
		}
		fmt.Printf("Metric: %v\n", timeseries.Metric.Type)
		for k, v := range timeseries.Metric.Labels {
			fmt.Printf("Label: %v=%v\n", k, v)
		}
		fmt.Printf("Resource: %v\n", timeseries.Resource.Type)
		for k, v := range timeseries.Resource.Labels {
			fmt.Printf("Label: %v=%v\n", k, v)
		}
		//fmt.Printf("\tNow: %.4f\n", timeseries.GetPoints()[0].GetValue().GetDoubleValue())
		//if len(timeseries.GetPoints()) > 1 {
		//	fmt.Printf("\t10 minutes ago: %.4f\n", timeseries.GetPoints()[1].GetValue().GetDoubleValue())
		//}

		for _, point := range timeseries.Points {
			fmt.Printf("Point: [%v-%v] = %v\n",
				point.Interval.StartTime.Seconds,
				point.Interval.EndTime.Seconds,
				point.GetValue().GetInt64Value())
		}
	}
}
