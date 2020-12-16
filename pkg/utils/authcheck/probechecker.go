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

// Package authcheck provides utilities to check authentication configuration for data plane resources.
// probechecker.go  utilities to perform a probe check for liviness and readiness.
package authcheck

import (
	"context"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

// DefaultProbeCheckPort is the default port for checking sync pool health.
const DefaultProbeCheckPort = 8080

type ProbeChecker struct {
	logger    *zap.Logger
	port      int
	authCheck AuthenticationCheck
}

// NewProbeChecker returns ProbeChecker with default probe checker port.
func NewProbeChecker(logger *zap.Logger, authType AuthType) ProbeChecker {
	return ProbeChecker{
		logger:    logger,
		port:      DefaultProbeCheckPort,
		authCheck: NewDefault(authType),
	}
}

// Start will initialize an http server and start to listen.
func (pc *ProbeChecker) Start(ctx context.Context) {
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(pc.port),
		Handler: pc,
	}

	go func() {
		pc.logger.Info("Starting probe checker...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			pc.logger.Error("the probe checker has stopped unexpectedly", zap.Error(err))
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(ctx); err != nil {
		pc.logger.Error("failed to shutdown the pubsub probe checker", zap.Error(err))
	}
}

// ServerHTTP will perform the authentication check if the request path is /healthz.
func (pc *ProbeChecker) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/healthz" {
		// Perform Authentication check.
		if err := pc.authCheck.Check(req.Context()); err != nil {
			pc.logger.Error("authentication check failed", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
