package lib

import (
	"context"
	"fmt"
	"strconv"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/test/lib/metrics"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

type TriggerMetricAssertion struct {
	ProjectID                   string
	BrokerNamespace             string
	BrokerName                  string
	StartTime                   time.Time
	CountPerTriggerWithRespCode map[TriggerAssertionKey]int64
	CountPerTriggerNoRespCode   map[TriggerAssertionKey]int64
}

type TriggerAssertionKey struct {
	Name string
	// If not provided, it means the assertion doesn't care about response code.
	RespCode int
}

func (a TriggerMetricAssertion) Assert(client *monitoring.MetricClient) error {
	if err := a.assertMetric(client, TriggerEventCountMetricType, a.CountPerTriggerWithRespCode, accumEventCount); err != nil {
		return err
	}
	if err := a.assertMetric(client, TriggerEventDispatchLatencyType, a.CountPerTriggerWithRespCode, accumDispatchLatency); err != nil {
		return err
	}
	if err := a.assertMetric(client, TriggerEventProcessingLatencyType, a.CountPerTriggerNoRespCode, accumProcessingLatency); err != nil {
		return err
	}
	return nil
}

func (a TriggerMetricAssertion) assertMetric(client *monitoring.MetricClient, metric string, wantCount map[TriggerAssertionKey]int64, accF func(map[TriggerAssertionKey]int64, *monitoringpb.TimeSeries) error) error {
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
	gotCount := make(map[TriggerAssertionKey]int64)
	for {
		ts, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if err := accF(gotCount, ts); err != nil {
			return err
		}
	}
	if diff := cmp.Diff(wantCount, gotCount); diff != "" {
		return fmt.Errorf("unexpected metric %q count (-want, +got) = %v", metric, diff)
	}
	return nil
}

func accumEventCount(accum map[TriggerAssertionKey]int64, ts *monitoringpb.TimeSeries) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	labels := ts.GetMetric().GetLabels()
	code, err := strconv.Atoi(labels["response_code"])
	if err != nil {
		return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
	}
	tn := TriggerAssertionKey{
		Name:     triggerName,
		RespCode: code,
	}
	accum[tn] = accum[tn] + metrics.SumCumulative(ts)
	return nil
}

func accumDispatchLatency(accum map[TriggerAssertionKey]int64, ts *monitoringpb.TimeSeries) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	labels := ts.GetMetric().GetLabels()
	code, err := strconv.Atoi(labels["response_code"])
	if err != nil {
		return fmt.Errorf("metric has invalid response code label: %v", ts.GetMetric())
	}
	tn := TriggerAssertionKey{
		Name:     triggerName,
		RespCode: code,
	}
	accum[tn] = accum[tn] + metrics.SumDistCount(ts)
	return nil
}

func accumProcessingLatency(accum map[TriggerAssertionKey]int64, ts *monitoringpb.TimeSeries) error {
	triggerName := ts.GetResource().GetLabels()["trigger_name"]
	tn := TriggerAssertionKey{
		Name: triggerName,
	}
	accum[tn] = accum[tn] + metrics.SumDistCount(ts)
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
