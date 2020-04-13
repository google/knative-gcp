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
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
)

func TestWatchAndSync(t *testing.T) {
	testProject := "test-project"
	targets := make(chan *config.TargetsConfig)
	defer close(targets)
	emptyTargetConfig := new(config.TargetsConfig)
	p, err := StartSyncPool(context.Background(), config.NewTargetsWatcher(targets), pool.WithProjectID(testProject))
	if err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}
	assertHandlers(t, p, emptyTargetConfig)

	targetConfig := &config.TargetsConfig{Brokers: make(map[string]*config.Broker)}
	// First add some brokers.
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			ns := fmt.Sprintf("ns-%d", i)
			bn := fmt.Sprintf("broker-%d", j)
			targetConfig.Brokers[fmt.Sprintf("%s/%s", ns, bn)] = &config.Broker{
				Namespace: ns,
				Name:      bn,
				Address:   "address",
				DecoupleQueue: &config.Queue{
					Topic:        fmt.Sprintf("t-%d-%d", i, j),
					Subscription: fmt.Sprintf("sub-%d-%d", i, j),
				},
			}
		}
	}
	targets <- targetConfig
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, targetConfig)

	newTargetConfig := &config.TargetsConfig{Brokers: make(map[string]*config.Broker)}
	// First add some brokers.
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			ns := fmt.Sprintf("ns-%d", i+2)
			bn := fmt.Sprintf("broker-%d", j+2)
			newTargetConfig.Brokers[fmt.Sprintf("%s/%s", ns, bn)] = &config.Broker{
				Namespace: ns,
				Name:      bn,
				Address:   "address",
				DecoupleQueue: &config.Queue{
					Topic:        fmt.Sprintf("t-%d-%d", i, j),
					Subscription: fmt.Sprintf("sub-%d-%d", i, j),
				},
			}
		}
	}
	targets <- newTargetConfig
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, newTargetConfig)

	targets <- emptyTargetConfig
	// Wait a short period for the handlers to be updated.
	<-time.After(time.Second)
	assertHandlers(t, p, emptyTargetConfig)
}

func assertHandlers(t *testing.T, p *SyncPool, c *config.TargetsConfig) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	for _, b := range c.GetBrokers() {
		wantHandlers[fmt.Sprintf("%s/%s", b.Namespace, b.Name)] = true
	}

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}

// TODO(yolocs): add semi-e2e tests for fanout pool.
