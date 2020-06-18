package lib

import (
	"context"
	"fmt"
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

type TriggerMetricNoRespCodeAssertion struct {
	TriggerMetricAssertion
}

type TriggerMetricWithRespCodeAssertion struct {
	TriggerMetricAssertion
	ResponseCode int
}

func (a TriggerMetricNoRespCodeAssertion) Assert(client *monitoring.MetricClient) error {
	return a.TriggerMetricAssertion.assertMetric(client, TriggerEventProcessingLatencyType, 0, accumProcessingLatency)
}

func (a TriggerMetricWithRespCodeAssertion) Assert(client *monitoring.MetricClient) error {
	if err := a.TriggerMetricAssertion.assertMetric(client, TriggerEventCountMetricType, a.ResponseCode, accumEventCount); err != nil {
		return err
	}
	if err := a.TriggerMetricAssertion.assertMetric(client, TriggerEventDispatchLatencyType, a.ResponseCode, accumDispatchLatency); err != nil {
		return err
	}
	return nil
}

func (a TriggerMetricAssertion) assertMetric(client *monitoring.MetricClient, metric string, respCode int, accF func(map[string]int64, *monitoringpb.TimeSeries, int) error) error {
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
		Filter:   a.StackdriverFilter(metric),
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
		if err := accF(gotCount, ts, respCode); err != nil {
			return err
		}
	}
	if diff := cmp.Diff(a.CountPerTrigger, gotCount); diff != "" {
		return fmt.Errorf("unexpected metric %q (resp code = %v) count (-want, +got) = %v", metric, respCode, diff)
	}
	return nil
}

func accumEventCount(accum map[string]int64, ts *monitoringpb.TimeSeries, respCode int) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	labels := ts.GetMetric().GetLabels()
	code, err := strconv.Atoi(labels["response_code"])
	if err != nil {
		return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
	}
	if code != respCode {
		// Matching response code.
		return nil
	}
	accum[triggerName] = accum[triggerName] + metrics.SumCumulative(ts)
	return nil
}

func accumDispatchLatency(accum map[string]int64, ts *monitoringpb.TimeSeries, respCode int) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	labels := ts.GetMetric().GetLabels()
	code, err := strconv.Atoi(labels["response_code"])
	if err != nil {
		return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
	}
	if code != respCode {
		// Matching response code.
		return nil
	}
	accum[triggerName] = accum[triggerName] + metrics.SumDistCount(ts)
	return nil
}

func accumProcessingLatency(accum map[string]int64, ts *monitoringpb.TimeSeries, respCode int) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	accum[triggerName] = accum[triggerName] + metrics.SumDistCount(ts)
	return nil
}

func (a TriggerMetricAssertion) StackdriverFilter(metric string) string {
	filter := map[string]interface{}{
		"metric.type":                   metric,
		"resource.type":                 TriggerMonitoredResourceType,
		"resource.label.namespace_name": a.BrokerNamespace,
		"resource.label.broker_name":    a.BrokerName,
	}
	return metrics.StringifyStackDriverFilter(filter)
}
