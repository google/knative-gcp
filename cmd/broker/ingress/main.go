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
	"github.com/google/knative-gcp/pkg/broker/ingress"
	"github.com/google/knative-gcp/pkg/utils"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	Port      int    `envconfig:"PORT" default:"8080"`
	ProjectID string `envconfig:"PROJECT_ID"`
}

// main creates and starts an ingress handler using default options.
// 1. It listens on port specified by "PORT" env var, or default 8080 if env var is not set
// 2. It reads "PROJECT_ID" env var for pubsub project. If the env var is empty, it retrieves project ID from
//    GCE metadata.
// 3. It expects broker configmap mounted at "/var/run/cloud-run-events/broker/targets"
func main() {
	// Since we pass nil, a default config with no error will be returned.
	cfg, _ := logging.NewConfigFromMap(nil)
	logger, _ := logging.NewLoggerFromConfig(cfg, "broker-ingress")
	ctx := signals.NewContext()
	ctx = logging.WithLogger(ctx, logger)

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logger.Desugar().Fatal("Failed to process env var", zap.Error(err))
	}
	projectID, err := utils.ProjectID(env.ProjectID)
	if err != nil {
		logger.Desugar().Fatal("Failed to get project id", zap.Error(err))
	}
	logger.Desugar().Info("Starting ingress handler", zap.Any("envConfig", env), zap.Any("Project ID", projectID))

	ingress, err := ingress.NewHandler(ctx, ingress.WithPort(env.Port), ingress.WithProjectID(projectID))
	if err != nil {
		logger.Desugar().Fatal("Unable to create ingress handler: ", zap.Error(err))
	}

	logger.Info("Starting ingress.", zap.Any("ingress", ingress))
	if err := ingress.Start(ctx); err != nil {
		logger.Desugar().Fatal("failed to start ingress: ", zap.Error(err))
	}
}
