/*
Copyright 2019 Google LLC

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
	"cloud.google.com/go/compute/metadata"
	"flag"
	"fmt"
	"github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/google/knative-gcp/pkg/reconciler/pullsubscription/resources"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"log"
)

const (
	component = "pullsubscription"
)

func main() {
	flag.Parse()

	startable := adapter.Adapter{}
	if err := envconfig.Process("", &startable); err != nil {
		panic(fmt.Sprintf("Failed to process env var: %s", err))
	}

	// Convert base64 encoded json logging.Config to logging.Config.
	loggingConfig, err := resources.Base64ToLoggingConfig(startable.LoggingConfigBase64)
	if err != nil {
		fmt.Printf("[ERROR] filed to process logging config: %s", err.Error())
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}

	logger, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(logger)
	ctx := logging.WithLogger(signals.NewContext(), logger)

	// Convert base64 encoded json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := resources.Base64ToMetricsOptions(startable.MetricsConfigBase64)
	if err != nil {
		logger.Fatalf("failed to process metrics options: %s", err.Error())
	}

	if metricsConfig == nil {
		logger.Fatal("invalid metric options: nil")
	}

	if err := metrics.UpdateExporter(*metricsConfig, logger); err != nil {
		log.Fatalf("Failed to create the metrics exporter: %s", err.Error())
	}

	reporter, err := metrics.NewStatsReporter()
	if err != nil {
		log.Fatalf("Failed to create metrics reporter: %s", err.Error())
	}
	startable.Reporter = reporter

	if startable.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		startable.Project = project
	}

	logger.Info("using project.", zap.String("project", startable.Project))

	logger.Info("Starting Pub/Sub Receive Adapter.", zap.Any("adapter", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
