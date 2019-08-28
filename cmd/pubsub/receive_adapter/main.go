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
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/xml"
	transporthttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/knative-gcp/pkg/pubsub/adapter"
	"github.com/google/knative-gcp/pkg/reconciler/pullsubscription/resources"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
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

	// Convert base64 encoded json logging.Config to logging.Config.
	loggingConfig, err := resources.Base64ToLoggingConfig(startable.LoggingConfigBase64)
	if err != nil {
		panic(err)
	}

	// Convert base64 encoded json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := resources.Base64ToMetricsOptions(startable.MetricsConfigBase64)
	if err != nil {
		panic(err)
	}

	logger, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	defer flush(logger)
	ctx := logging.WithLogger(signals.NewContext(), logger)

	mainMetrics(logger, metricsConfig)

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

func mainMetrics(logger *zap.SugaredLogger, opts *metrics.ExporterOptions) {
	if err := metrics.UpdateExporter(*opts, logger); err != nil {
		log.Fatalf("Failed to create the metrics exporter: %v", err)
	}

	// Register the views
	if err := view.Register(
		client.LatencyView,
		transporthttp.LatencyView,
		json.LatencyView,
		xml.LatencyView,
		datacodec.LatencyView,
		adapter.LatencyView,
	); err != nil {
		log.Fatalf("Failed to register views: %v", err)
	}

	view.SetReportingPeriod(2 * time.Second)
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}
