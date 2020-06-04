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

package handler

import (
	"context"
	"fmt"
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
			returnErr:  true,
			syncCalled: make(chan struct{}, 1),
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
			returnErr:  false,
			syncCalled: make(chan struct{}, 1),
		}
		ch := make(chan struct{})
		if _, err := StartSyncPool(ctx, syncPool, ch); err != nil {
			t.Errorf("StartSyncPool got unexpected error: %v", err)
		}
		syncPool.verifySyncOnceCalled(t)

		ch <- struct{}{}
		syncPool.verifySyncOnceCalled(t)
	})
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
