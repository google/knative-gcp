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
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSyncPool(t *testing.T) {
	wantErr := fmt.Errorf("error returned from fakeSyncPool")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("StartSyncPool returns error", func(t *testing.T) {
		syncPool := &fakeSyncPool{
			returnErr: true,
		}
		_, gotErr := StartSyncPool(ctx, syncPool, make(chan struct{}))
		if gotErr == nil {
			t.Error("StartSyncPool got unexpected result")
		}
		if diff := cmp.Diff(wantErr.Error(), gotErr.Error()); diff != "" {
			t.Errorf("StartSyncPool (-want,+got): %v", diff)
		}
	})

	t.Run("Work done with StartSyncPool", func(t *testing.T) {
		syncPool := &fakeSyncPool{
			returnErr: false,
		}
		ch := make(chan struct{})
		if _, err := StartSyncPool(ctx, syncPool, ch); err != nil {
			t.Errorf("StartSyncPool got unexpected error: %v", err)
		}
		if syncPool.syncCount.Load() != 1 {
			t.Errorf("SyncOnce was called more than once")
		}
		ch <- struct{}{}
		time.Sleep(500 * time.Millisecond)
		if syncPool.syncCount.Load() != 2 {
			t.Errorf("SyncOnce was not called twice")
		}
	})
}

type fakeSyncPool struct {
	returnErr bool
	syncCount atomic.Value
}

func (p *fakeSyncPool) SyncOnce(ctx context.Context) error {
	if p.syncCount.Load() == nil {
		p.syncCount.Store(0)
	}
	count := p.syncCount.Load().(int)
	count++
	p.syncCount.Store(count)
	if p.returnErr {
		return fmt.Errorf("error returned from fakeSyncPool")
	}
	return nil
}
