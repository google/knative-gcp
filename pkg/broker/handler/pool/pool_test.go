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

package pool

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSyncPool(t *testing.T) {
	t.Run("StartSyncPool returns error", func(t *testing.T) {
		wantErr := fmt.Errorf("error returned from fakeSyncPool")
		syncPool := &fakeSyncPool{
			returnErr:  true,
			syncCalled: make(chan struct{}, 1),
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, gotErr := StartSyncPool(ctx, syncPool, make(chan struct{}), 30*time.Second)
		if gotErr == nil {
			t.Error("StartSyncPool got unexpected result")
		}
		if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
			t.Errorf("StartSyncPool (-want,+got): %v", diff)
		}
	})

	t.Run("Work done with StartSyncPool", func(t *testing.T) {
		syncPool := &fakeSyncPool{
			returnErr:  false,
			syncCalled: make(chan struct{}, 1),
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan struct{})
		if _, err := StartSyncPool(ctx, syncPool, ch, time.Second); err != nil {
			t.Errorf("StartSyncPool got unexpected error: %v", err)
		}
		syncPool.verifySyncOnceCalled(t)
		// Make sure the health checker is up.
		time.Sleep(500 * time.Millisecond)

		ch <- struct{}{}
		syncPool.verifySyncOnceCalled(t)
		assertHealthCheckResult(t, true)

		// Intentionally causing a unhealth check.
		time.Sleep(time.Second)
		assertHealthCheckResult(t, false)
	})
}

func assertHealthCheckResult(t *testing.T, ok bool) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/healthz", nil)
	if err != nil {
		t.Fatalf("Failed to create health check request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Failed to execute health check: %v", err)
		if ok {
			t.Errorf("health check result ok got=%v, want=%v", !ok, ok)
		}
		return
	}
	if ok != (resp.StatusCode == http.StatusOK) {
		t.Logf("Got health check status code: %v", resp.StatusCode)
		t.Errorf("health check result ok got=%v, want=%v", !ok, ok)
	}
}

type fakeSyncPool struct {
	returnErr  bool
	syncCalled chan struct{}
}

func (p *fakeSyncPool) verifySyncOnceCalled(t *testing.T) {
	select {
	case <-time.After(500 * time.Millisecond):
		t.Errorf("SyncOnce was not called before timeout")
	case <-p.syncCalled:
		// Good.
	}
}

func (p *fakeSyncPool) SyncOnce(ctx context.Context) error {
	p.syncCalled <- struct{}{}
	if p.returnErr {
		return fmt.Errorf("error returned from fakeSyncPool")
	}
	return nil
}
