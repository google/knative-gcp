/*
Copyright 2019 Google LLC.

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

package testing

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"
	"knative.dev/pkg/metrics/metricskey"
)

type Trigger struct {
	Namespace string
	Trigger   string
	Broker    string
}

type deliveryKey struct {
	Trigger Trigger
	Code    int
}

type Tags map[string]string

type ExpectDelivery struct {
	TriggerTags     map[Trigger]Tags
	ProcessingCount map[Trigger]int64
	DeliveryCount   map[deliveryKey]int64
	TimeoutCount    map[Trigger]int64
}

func NewExpectDelivery() ExpectDelivery {
	return ExpectDelivery{
		TriggerTags:     make(map[Trigger]Tags),
		ProcessingCount: make(map[Trigger]int64),
		DeliveryCount:   make(map[deliveryKey]int64),
		TimeoutCount:    make(map[Trigger]int64),
	}
}

type metric struct {
	Tags     map[string]string
	Count    int64
	Count500 int64
}

func (e ExpectDelivery) AddTrigger(t *testing.T, trigger Trigger, expectTags Tags) {
	if _, ok := e.TriggerTags[trigger]; ok {
		t.Fatalf("trigger %q already defined", trigger)
	}
	e.TriggerTags[trigger] = expectTags
}

func (e ExpectDelivery) ExpectProcessing(t *testing.T, trigger Trigger) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	e.ProcessingCount[trigger] = e.ProcessingCount[trigger] + 1
}

func (e ExpectDelivery) ExpectDelivery(t *testing.T, trigger Trigger, code int) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	key := deliveryKey{Trigger: trigger, Code: code}
	e.DeliveryCount[key] = e.DeliveryCount[key] + 1
}

func (e ExpectDelivery) ExpectTimeout(t *testing.T, trigger Trigger) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	e.TimeoutCount[trigger] = e.TimeoutCount[trigger] + 1
}

func (e ExpectDelivery) Expect200(t *testing.T, trigger Trigger) {
	e.ExpectProcessing(t, trigger)
	e.ExpectDelivery(t, trigger, 200)
}

func (e ExpectDelivery) Verify(t *testing.T) {
	t.Helper()
	var err error
	timeout := time.After(2 * time.Second)
	for {
		err = e.attemptVerify()
		if err == nil {
			return
		}
		// Retry since stats are updated asynchronously
		select {
		case <-timeout:
			t.Fatal(err)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (e ExpectDelivery) attemptVerify() error {
	if err := e.verifyProcessing(); err != nil {
		return err
	}
	if err := e.verifyDelivery("event_count"); err != nil {
		return err
	}
	if err := e.verifyDelivery("event_dispatch_latencies"); err != nil {
		return err
	}
	if err := e.verifyTimeout(); err != nil {
		return err
	}
	return nil
}

func (e ExpectDelivery) verifyDelivery(viewName string) error {
	got := make(map[deliveryKey]int64)
	for _, p := range metricproducer.GlobalManager().GetAll() {
		for _, m := range p.Read() {
			if m.Resource.Type != metricskey.ResourceTypeKnativeTrigger {
				continue
			}
			if m.Descriptor.Name != viewName {
				continue
			}
			t := Trigger{
				Namespace: m.Resource.Labels[metricskey.LabelNamespaceName],
				Trigger:   m.Resource.Labels[metricskey.LabelTriggerName],
				Broker:    m.Resource.Labels[metricskey.LabelBrokerName],
			}
			for _, ts := range m.TimeSeries {
				tags := make(map[string]string)
				for i, k := range m.Descriptor.LabelKeys {
					if v := ts.LabelValues[i]; v.Present {
						tags[k.Key] = v.Value
					}
				}

				if tags["response_code"] == "" {
					// Skip time out record which doesn't have response code.
					continue
				}
				if code, err := strconv.Atoi(tags["response_code"]); err != nil {
					return err
				} else {
					got[deliveryKey{Trigger: t, Code: code}] = getCount(ts)
				}
				ignoreCodeTags := cmpopts.IgnoreMapEntries(func(k string, v string) bool {
					return k == "response_code" || k == "response_code_class"
				})
				if diff := cmp.Diff(e.TriggerTags[t], Tags(tags), ignoreCodeTags); diff != "" {
					return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
				}
			}
		}
	}

	if diff := cmp.Diff(e.DeliveryCount, got); diff != "" {
		return fmt.Errorf("unexpected %s measurement count (-want, +got) = %v", viewName, diff)
	}
	return nil
}

func (e ExpectDelivery) verifyProcessing() error {
	got := make(map[Trigger]int64)
	for _, p := range metricproducer.GlobalManager().GetAll() {
		for _, m := range p.Read() {
			if m.Resource.Type != metricskey.ResourceTypeKnativeTrigger {
				continue
			}
			if m.Descriptor.Name != "event_processing_latencies" {
				continue
			}
			t := Trigger{
				Namespace: m.Resource.Labels[metricskey.LabelNamespaceName],
				Trigger:   m.Resource.Labels[metricskey.LabelTriggerName],
				Broker:    m.Resource.Labels[metricskey.LabelBrokerName],
			}
			for _, ts := range m.TimeSeries {
				tags := make(map[string]string)
				for i, k := range m.Descriptor.LabelKeys {
					if v := ts.LabelValues[i]; v.Present {
						tags[k.Key] = v.Value
					}
				}
				got[t] = getCount(ts)
				if diff := cmp.Diff(e.TriggerTags[t], Tags(tags)); diff != "" {
					return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
				}
			}
		}
	}

	if diff := cmp.Diff(e.ProcessingCount, got); diff != "" {
		return fmt.Errorf("unexpected event_processing_latencies measurement count (-want, +got) = %v", diff)
	}
	return nil
}

func (e ExpectDelivery) verifyTimeout() error {
	got := make(map[Trigger]int64)
	for _, p := range metricproducer.GlobalManager().GetAll() {
		for _, m := range p.Read() {
			if m.Resource.Type != metricskey.ResourceTypeKnativeTrigger {
				continue
			}
			if m.Descriptor.Name != "event_dispatch_latencies" {
				continue
			}
			t := Trigger{
				Namespace: m.Resource.Labels[metricskey.LabelNamespaceName],
				Trigger:   m.Resource.Labels[metricskey.LabelTriggerName],
				Broker:    m.Resource.Labels[metricskey.LabelBrokerName],
			}
			for _, ts := range m.TimeSeries {
				tags := make(map[string]string)
				for i, k := range m.Descriptor.LabelKeys {
					if v := ts.LabelValues[i]; v.Present {
						tags[k.Key] = v.Value
					}
				}
				if tags["response_code"] != "" {
					continue
				}
				got[t] = getCount(ts)
				if diff := cmp.Diff(e.TriggerTags[t], Tags(tags)); diff != "" {
					return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
				}
			}
		}
	}

	if diff := cmp.Diff(e.TimeoutCount, got); diff != "" {
		return fmt.Errorf("unexpected timeout event_dispatch_latencies measurement count (-want, +got) = %v", diff)
	}
	return nil
}

type counter struct {
	count int64
}

func (c *counter) VisitFloat64Value(f float64) {
	c.count += 1
}

func (c *counter) VisitInt64Value(i int64) {
	c.count = i
}

func (c *counter) VisitDistributionValue(d *metricdata.Distribution) {
	c.count += d.Count
}

func (c *counter) VisitSummaryValue(d *metricdata.Summary) {
	c.count = d.Count
}

func getCount(ts *metricdata.TimeSeries) int64 {
	c := new(counter)
	for _, p := range ts.Points {
		p.ReadValue(c)
	}
	return c.count
}
