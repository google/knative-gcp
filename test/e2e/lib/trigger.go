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

type TriggerMetricAssertion struct {
	ProjectID       string
	BrokerNamespace string
	BrokerName      string
	StartTime       time.Time
	CountPerTrigger map[string]int64
}

func (a TriggerMetricAssertion) Assert(client *monitoring.MetricClient) error {
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
		triggerName := ts.GetResource().GetLabels()["trigger_name"]
		labels := ts.GetMetric().GetLabels()
		code, err := strconv.Atoi(labels["response_code"])
		if err != nil {
			return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
		}
		if code != http.StatusAccepted && code != http.StatusOK {
			return fmt.Errorf("metric has unexpected response code: %v", ts.GetMetric())
		}
		gotCount[triggerName] = gotCount[triggerName] + metrics.SumCumulative(ts)
	}
	if diff := cmp.Diff(a.CountPerTrigger, gotCount); diff != "" {
		return fmt.Errorf("unexpected broker metric count (-want, +got) = %v", diff)
	}
	return nil
}

func (a TriggerMetricAssertion) StackdriverFilter() string {
	filter := map[string]interface{}{
		"metric.type":                   TriggerEventCountMetricType,
		"resource.type":                 TriggerMonitoredResourceType,
		"resource.label.namespace_name": a.BrokerNamespace,
		"resource.label.broker_name":    a.BrokerName,
	}
	return metrics.StringifyStackDriverFilter(filter)
}
