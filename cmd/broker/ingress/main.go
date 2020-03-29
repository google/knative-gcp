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

package main

import (
	"go.uber.org/zap"

	"github.com/google/knative-gcp/pkg/broker/ingress"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

// main creates and starts an ingress handler using default options.
// 1. it listens on port 8080
// 2. it reads "GOOGLE_CLOUD_PROJECT" env var for pubsub project.
// 3. it expects broker configmap mounted at "/var/run/cloud-run-events/broker/targets"
func main() {
	// Since we pass nil, a default config with no error will be returned.
	cfg, _ := logging.NewConfigFromMap(nil)
	logger, _ := logging.NewLoggerFromConfig(cfg, "broker-ingress")
	ctx := signals.NewContext()
	ctx = logging.WithLogger(ctx, logger)

	ingress, err := ingress.NewHandler(ctx)
	if err != nil {
		logger.Desugar().Fatal("Unable to create ingress handler: ", zap.Error(err))
	}

	logger.Info("Starting ingress.", zap.Any("ingress", ingress))
	if err := ingress.Start(ctx); err != nil {
		logger.Desugar().Fatal("failed to start ingress: ", zap.Error(err))
	}
}
