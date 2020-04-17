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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSyncPool(t *testing.T) {
	wantErr := fmt.Errorf("error returned from fakeSyncPool")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	syncPool := &FakeSyncPool{}

	t.Run("StartSyncPool returns error", func(t *testing.T) {
		// Add some brokers with their targets.
		tctx := context.WithValue(ctx, "error", "true")
		_, gotErr := StartSyncPool(tctx, syncPool, make(chan struct{}))
		if gotErr == nil {
			t.Error("StartSyncPool got unexpected result")
		}
		if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
			t.Errorf("StartSyncPool (-want,+got): %v", diff)
		}
	})

	t.Run("Work done with StartSyncPool", func(t *testing.T) {
		// Add some brokers with their targets.
		tctx := context.WithValue(ctx, "error", "false")
		if _, err := StartSyncPool(tctx, syncPool, make(chan struct{})); err != nil {
			t.Errorf("StartSyncPool got unexpected error: %v", err)
		}
	})
}

type FakeSyncPool struct{}

func (p *FakeSyncPool) SyncOnce(ctx context.Context) error {
	if ctx.Value("error") == "true" {
		return fmt.Errorf("error returned from fakeSyncPool")
	}
	return nil
}
