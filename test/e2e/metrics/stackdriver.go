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

	"cloud.google.com/go/monitoring/apiv3"
	"github.com/golang/protobuf/ptypes/duration"
	googlepb "github.com/golang/protobuf/ptypes/timestamp"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// TODO upstream to knative/pkg

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

func WithStackDriverFilter(filter string) StackDriverListTimeSeriesRequestOption {
	return func(r *monitoringpb.ListTimeSeriesRequest) {
		r.Filter = filter
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
		r.Aggregation.AlignmentPeriod = &duration.Duration{Seconds: seconds}
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

func StringifyStackDriverFilter(filter map[string]interface{}) string {
	var sb strings.Builder
	for k, v := range filter {
		sb.WriteString(fmt.Sprintf("%s=\"%v\" ", k, v))
	}
	return strings.TrimSuffix(sb.String(), " ")
}
