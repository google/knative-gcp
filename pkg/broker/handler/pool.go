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
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/knative-gcp/pkg/logging"
	"go.uber.org/zap"
)

const (
	// DefaultProbeCheckPort is the default port for checking sync pool health.
	DefaultProbeCheckPort = 8080
)

type SyncPool interface {
	SyncOnce(ctx context.Context) error
}

type probeChecker struct {
	mux              sync.RWMutex
	lastReportTime   time.Time
	maxStaleDuration time.Duration
	port             int
}

func (c *probeChecker) reportHealth() {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.lastReportTime = time.Now()
}

func (c *probeChecker) lastTime() time.Time {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return c.lastReportTime
}

func (c *probeChecker) start(ctx context.Context) {
	c.reportHealth()
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(c.port),
		Handler: c,
	}

	go func() {
		logging.FromContext(ctx).Info("Starting the sync pool probe checker...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.FromContext(ctx).Error("the sync pool probe checker has stopped unexpectedly", zap.Error(err))
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(ctx); err != nil {
		logging.FromContext(ctx).Error("failed to shutdown the sync pool probe checker", zap.Error(err))
	}
}

func (c *probeChecker) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/healthz" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// Zero maxStaleDuration means infinite.
	if c.maxStaleDuration == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	if time.Now().Sub(c.lastTime()) > c.maxStaleDuration {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// StartSyncPool starts the sync pool.
func StartSyncPool(
	ctx context.Context,
	syncPool SyncPool,
	syncSignal <-chan struct{},
	maxStaleDuration time.Duration,
	probeCheckPort int,
) (SyncPool, error) {

	if err := syncPool.SyncOnce(ctx); err != nil {
		return nil, err
	}
	c := &probeChecker{
		maxStaleDuration: maxStaleDuration,
		port:             probeCheckPort,
	}
	go c.start(ctx)
	if syncSignal != nil {
		go watch(ctx, syncPool, syncSignal, c)
	}
	return syncPool, nil
}

func watch(ctx context.Context, syncPool SyncPool, syncSignal <-chan struct{}, c *probeChecker) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-syncSignal:
			if err := syncPool.SyncOnce(ctx); err != nil {
				// Currently we don't really expect errors from SyncOnce.
				logging.FromContext(ctx).Error("failed to sync handlers pool on watch signal", zap.Error(err))
			} else {
				logging.FromContext(ctx).Debug("successfully synced handlers pool on watch signal")
				c.reportHealth()
			}
		}
	}
}
