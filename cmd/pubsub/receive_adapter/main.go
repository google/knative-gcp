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
	"flag"
	"fmt"
	"log"

	"cloud.google.com/go/compute/metadata"
	"github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
)

const (
	component = "PullSubscription::ReceiveAdapter"
)

func main() {
	flag.Parse()

	startable := adapter.Adapter{}
	if err := envconfig.Process("", &startable); err != nil {
		panic(fmt.Sprintf("Failed to process env var: %s", err))
	}

	// Convert json logging.Config to logging.Config.
	loggingConfig, err := logging.JsonToLoggingConfig(startable.LoggingConfigJson)
	if err != nil {
		fmt.Printf("[ERROR] filed to process logging config: %s", err.Error())
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}

	sl, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := sl.Desugar()
	defer flush(logger)
	ctx := logging.WithLogger(signals.NewContext(), logger.Sugar())

	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JsonToMetricsOptions(startable.MetricsConfigJson)
	if err != nil {
		logger.Error("Failed to process metrics options", zap.Error(err))
	}

	mainMetrics(logger, metricsConfig)

	if startable.Project == "" {
		project, err := metadata.ProjectID()
		if err != nil {
			logger.Fatal("failed to find project id. ", zap.Error(err))
		}
		startable.Project = project
	}

	logger.Info("Starting Pub/Sub Receive Adapter.", zap.Any("adapter", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}

func mainMetrics(logger *zap.Logger, opts *metrics.ExporterOptions) {
	if opts == nil {
		logger.Info("metrics disabled")
		return
	}

	if err := metrics.UpdateExporter(*opts, logger.Sugar()); err != nil {
		log.Fatalf("Failed to create the metrics exporter: %v", err)
	}

	// TODO metrics are API surface, so make sure we need to expose this before doing so.
	//  These seem to be private ones and more profiling related ones.
	//  Commenting them for now, as we will use pkg/source stats_reporter.
	// Register the views.
	//if err := view.Register(
	//	client.LatencyView,
	//	transporthttp.LatencyView,
	//	json.LatencyView,
	//	xml.LatencyView,
	//	datacodec.LatencyView,
	//  adapter.LatencyView,
	//); err != nil {
	//	log.Fatalf("Failed to register views: %v", err)
	//}
	//
	//view.SetReportingPeriod(2 * time.Second)
}

func flush(logger *zap.Logger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
