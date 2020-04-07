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

package fanout

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
)

func TestWatchAndSync(t *testing.T) {
	testProject := "test-project"
	signal := make(chan struct{})
	targets := memory.NewEmptyTargets()
	p, err := StartSyncPool(context.Background(), targets,
		pool.WithProjectID(testProject),
		pool.WithSyncSignal(signal),
	)
	if err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}
	assertHandlers(t, p, targets)

	// First add some brokers.
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			ns := fmt.Sprintf("ns-%d", i)
			bn := fmt.Sprintf("broker-%d", j)
			targets.MutateBroker(ns, bn, func(bm config.BrokerMutation) {
				bm.SetAddress("address")
				bm.SetDecoupleQueue(&config.Queue{
					Topic:        fmt.Sprintf("t-%d-%d", i, j),
					Subscription: fmt.Sprintf("sub-%d-%d", i, j),
				})
			})
		}
	}
	signal <- struct{}{}
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, targets)

	// Delete old and add new.
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			ns := fmt.Sprintf("ns-%d", i)
			bn := fmt.Sprintf("broker-%d", j)
			targets.MutateBroker(ns, bn, func(bm config.BrokerMutation) {
				bm.Delete()
			})
			ns2 := fmt.Sprintf("ns-%d", i+2)
			bn2 := fmt.Sprintf("broker-%d", j+2)
			targets.MutateBroker(ns2, bn2, func(bm config.BrokerMutation) {
				bm.SetAddress("address")
				bm.SetDecoupleQueue(&config.Queue{
					Topic:        fmt.Sprintf("t-%d-%d", i+2, j+2),
					Subscription: fmt.Sprintf("sub-%d-%d", i+2, j+2),
				})
			})
		}
	}
	signal <- struct{}{}
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, targets)

	// clean up all brokers
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			ns2 := fmt.Sprintf("ns-%d", i+2)
			bn2 := fmt.Sprintf("broker-%d", j+2)
			targets.MutateBroker(ns2, bn2, func(bm config.BrokerMutation) {
				bm.Delete()
			})
		}
	}
	signal <- struct{}{}
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, targets)
}

func assertHandlers(t *testing.T, p *SyncPool, targets config.Targets) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	targets.RangeBrokers(func(b *config.Broker) bool {
		wantHandlers[config.BrokerKey(b.Namespace, b.Name)] = true
		return true
	})

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}

// TODO(yolocs): add semi-e2e tests for fanout pool.
