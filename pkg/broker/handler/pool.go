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

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/logging"
)

type SyncPool interface {
	SyncOnce(ctx context.Context) error
}

// StartSyncPool starts the sync pool.
func StartSyncPool(ctx context.Context, syncPool SyncPool, syncSignal <-chan struct{}) (SyncPool, error) {
	if err := syncPool.SyncOnce(ctx); err != nil {
		return nil, err
	}
	if syncSignal != nil {
		go watch(ctx, syncPool, syncSignal)
	}
	return syncPool, nil
}

func watch(ctx context.Context, syncPool SyncPool, syncSignal <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-syncSignal:
			if err := syncPool.SyncOnce(ctx); err != nil {
				logging.FromContext(ctx).Error("failed to sync handlers pool on watch signal", zap.Error(err))
			}
		}
	}
}
