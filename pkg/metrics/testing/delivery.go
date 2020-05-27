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
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opencensus.io/stats/view"
)

type deliveryKey struct {
	Trigger string
	Code    int
}

type Tags map[string]string

type ExpectDelivery struct {
	TriggerTags     map[string]Tags
	ProcessingCount map[string]int64
	DeliveryCount   map[deliveryKey]int64
}

func NewExpectDelivery() ExpectDelivery {
	return ExpectDelivery{
		TriggerTags:     make(map[string]Tags),
		ProcessingCount: make(map[string]int64),
		DeliveryCount:   make(map[deliveryKey]int64),
	}
}

type metric struct {
	Tags     map[string]string
	Count    int64
	Count500 int64
}

func (e ExpectDelivery) AddTrigger(t *testing.T, trigger string, expectTags Tags) {
	if _, ok := e.TriggerTags[trigger]; ok {
		t.Fatalf("trigger %q already defined", trigger)
	}
	e.TriggerTags[trigger] = expectTags
}

func (e ExpectDelivery) ExpectProcessing(t *testing.T, trigger string) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	e.ProcessingCount[trigger] = e.ProcessingCount[trigger] + 1
}

func (e ExpectDelivery) ExpectDelivery(t *testing.T, trigger string, code int) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	key := deliveryKey{Trigger: trigger, Code: code}
	e.DeliveryCount[key] = e.DeliveryCount[key] + 1
}

func (e ExpectDelivery) Expect200(t *testing.T, trigger string) {
	e.ExpectProcessing(t, trigger)
	e.ExpectDelivery(t, trigger, 200)
}

func (e ExpectDelivery) Verify(t *testing.T) {
	t.Helper()
	e.verifyProcessing(t)
	e.verifyDelivery(t, "event_count")
	e.verifyDelivery(t, "event_dispatch_latencies")
}

func verifyMetricCounts(t *testing.T, want map[string]*metric) {
}

func (e ExpectDelivery) verifyDelivery(t *testing.T, viewName string) {
	t.Helper()
	rows, err := view.RetrieveData(viewName)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[deliveryKey]int64)
	for _, row := range rows {
		tags := make(map[string]string)
		for _, t := range row.Tags {
			tags[t.Key.Name()] = t.Value
		}
		trigger, ok := tags["trigger_name"]
		if !ok {
			t.Errorf("missing trigger_name tag for row: %v", row)
			return
		}
		if code, err := strconv.Atoi(tags["response_code"]); err != nil {
			t.Errorf("invalid response code in tags: %v", tags)
			return
		} else {
			got[deliveryKey{Trigger: trigger, Code: code}] = getCount(t, row)
		}

		ignoreCodeTags := cmpopts.IgnoreMapEntries(func(k string, v string) bool {
			return k == "response_code" || k == "response_code_class"
		})
		if diff := cmp.Diff(e.TriggerTags[trigger], Tags(tags), ignoreCodeTags); diff != "" {
			t.Errorf("unexpected tags (-want, +got) = %v", diff)
		}
	}

	if diff := cmp.Diff(e.DeliveryCount, got); diff != "" {
		t.Errorf("unexpected event_processing_latencies measurement count (-want, +got) = %v", diff)
	}
}

func (e ExpectDelivery) verifyProcessing(t *testing.T) {
	t.Helper()
	rows, err := view.RetrieveData("event_processing_latencies")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]int64)
	for _, row := range rows {
		tags := make(map[string]string)
		for _, t := range row.Tags {
			tags[t.Key.Name()] = t.Value
		}
		trigger, ok := tags["trigger_name"]
		if !ok {
			t.Errorf("missing trigger_name tag for row: %v", row)
			return
		}
		if diff := cmp.Diff(e.TriggerTags[trigger], Tags(tags)); diff != "" {
			t.Errorf("unexpected tags (-want, +got) = %v", diff)
		}
		got[trigger] = getCount(t, row)
	}

	if diff := cmp.Diff(e.ProcessingCount, got); diff != "" {
		t.Errorf("unexpected event_processing_latencies measurement count (-want, +got) = %v", diff)
	}
}

func getCount(t *testing.T, row *view.Row) int64 {
	switch data := row.Data.(type) {
	case *view.CountData:
		return data.Value
	case *view.DistributionData:
		return data.Count
	default:
		t.Fatalf("unexpected metric type: %v", row)
		return 0
	}
}
