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

package utils

import (
	"context"
	nethttp "net/http"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

// ActionFunc represents a function which is called during a liveness probe. If
// it returns a non-nil error, the probe is considered unsuccessful.
type ActionFunc func(context.Context) error

// LivenessChecker executes each of the ActionFuncs upon each liveness probe.
type LivenessChecker struct {
	ActionFuncs []ActionFunc
}

// AddActionFunc appends an ActionFunc to the list of functions to be called
// by the liveness checker during each liveness probe.
func (c *LivenessChecker) AddActionFunc(f ActionFunc) {
	c.ActionFuncs = append(c.ActionFuncs, f)
}

// LivenessHandlerFunc returns the HTTP handler for probe helper liveness checks.
func (c *LivenessChecker) LivenessHandlerFunc(ctx context.Context) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, req *nethttp.Request) {
		// Only accept liveness requests along the 'healthz' path
		if req.URL.Path != "/healthz" {
			logging.FromContext(ctx).Warnw("Invalid liveness check request path", zap.String("path", req.URL.Path))
			w.WriteHeader(nethttp.StatusNotFound)
			return
		}
		// Apply every ActionFunc, do not interrupt when encountering an error
		var totalErr error
		for _, f := range c.ActionFuncs {
			if err := f(ctx); err != nil {
				totalErr = multierr.Append(totalErr, err)
			}
		}
		if totalErr != nil {
			// If any error was encountered, declare liveness failed
			logging.FromContext(ctx).Infow("Liveness check failed", zap.Error(totalErr))
			w.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		logging.FromContext(ctx).Info("Liveness check succeeded")
		w.WriteHeader(nethttp.StatusOK)
	}
}
