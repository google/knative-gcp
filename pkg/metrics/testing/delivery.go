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
	TimeoutCount    map[string]int64
}

func NewExpectDelivery() ExpectDelivery {
	return ExpectDelivery{
		TriggerTags:     make(map[string]Tags),
		ProcessingCount: make(map[string]int64),
		DeliveryCount:   make(map[deliveryKey]int64),
		TimeoutCount:    make(map[string]int64),
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

func (e ExpectDelivery) ExpectTimeout(t *testing.T, trigger string) {
	if _, ok := e.TriggerTags[trigger]; !ok {
		t.Fatalf("trigger %q not defined", trigger)
	}
	e.TimeoutCount[trigger] = e.TimeoutCount[trigger] + 1
}

func (e ExpectDelivery) Expect200(t *testing.T, trigger string) {
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
	rows, err := view.RetrieveData(viewName)
	if err != nil {
		return err
	}
	got := make(map[deliveryKey]int64)
	for _, row := range rows {
		tags := make(map[string]string)
		for _, t := range row.Tags {
			tags[t.Key.Name()] = t.Value
		}
		trigger, ok := tags["trigger_name"]
		if !ok {
			return fmt.Errorf("missing trigger_name tag for row: %v", row)
		}
		if tags["response_code"] == "" {
			// Skip time out record which doesn't have response code.
			continue
		} else if code, err := strconv.Atoi(tags["response_code"]); err != nil {
			return fmt.Errorf("invalid response code in tags: %v", tags)
		} else {
			if got[deliveryKey{Trigger: trigger, Code: code}], err = getCount(row); err != nil {
				return err
			}
		}

		ignoreCodeTags := cmpopts.IgnoreMapEntries(func(k string, v string) bool {
			return k == "response_code" || k == "response_code_class"
		})
		if diff := cmp.Diff(e.TriggerTags[trigger], Tags(tags), ignoreCodeTags); diff != "" {
			return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
		}
	}

	if diff := cmp.Diff(e.DeliveryCount, got); diff != "" {
		return fmt.Errorf("unexpected %s measurement count (-want, +got) = %v", viewName, diff)
	}
	return nil
}

func (e ExpectDelivery) verifyProcessing() error {
	rows, err := view.RetrieveData("event_processing_latencies")
	if err != nil {
		return err
	}
	got := make(map[string]int64)
	for _, row := range rows {
		tags := make(map[string]string)
		for _, t := range row.Tags {
			tags[t.Key.Name()] = t.Value
		}
		trigger, ok := tags["trigger_name"]
		if !ok {
			return fmt.Errorf("missing trigger_name tag for row: %v", row)
		}
		if diff := cmp.Diff(e.TriggerTags[trigger], Tags(tags)); diff != "" {
			return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
		}
		if got[trigger], err = getCount(row); err != nil {
			return err
		}
	}

	if diff := cmp.Diff(e.ProcessingCount, got); diff != "" {
		return fmt.Errorf("unexpected event_processing_latencies measurement count (-want, +got) = %v", diff)
	}
	return nil
}

func (e ExpectDelivery) verifyTimeout() error {
	rows, err := view.RetrieveData("event_dispatch_latencies")
	if err != nil {
		return err
	}
	got := make(map[string]int64)
	for _, row := range rows {
		tags := make(map[string]string)
		for _, t := range row.Tags {
			tags[t.Key.Name()] = t.Value
		}
		trigger, ok := tags["trigger_name"]
		if !ok {
			return fmt.Errorf("missing trigger_name tag for row: %v", row)
		}

		if tags["response_code"] != "" {
			continue
		}

		if diff := cmp.Diff(e.TriggerTags[trigger], Tags(tags)); diff != "" {
			return fmt.Errorf("unexpected tags (-want, +got) = %v", diff)
		}
		if got[trigger], err = getCount(row); err != nil {
			return err
		}
	}

	if diff := cmp.Diff(e.TimeoutCount, got); diff != "" {
		return fmt.Errorf("unexpected timeout event_dispatch_latencies measurement count (-want, +got) = %v", diff)
	}
	return nil
}

func getCount(row *view.Row) (int64, error) {
	switch data := row.Data.(type) {
	case *view.CountData:
		return data.Value, nil
	case *view.DistributionData:
		return data.Count, nil
	default:
		return 0, fmt.Errorf("unexpected metric type: %v", row)
	}
}
