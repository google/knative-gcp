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
	"log"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/xml"
	transporthttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/knative-gcp/pkg/pubsub/adapter"
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

	sl, _ := logging.NewLogger("", "INFO") // TODO: use logging config map.
	logger := sl.Desugar()
	defer logger.Sync()

	ctx := logging.WithLogger(signals.NewContext(), logger.Sugar())

	startable := adapter.Adapter{}
	if err := envconfig.Process("", &startable); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}

	mainMetrics(sl)

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

func mainMetrics(logger *zap.SugaredLogger) {
	cfg := map[string]string{
		"metrics.backend-destination": "prometheus", // TODO: hard code for now while we test.
	}

	if err := metrics.UpdateExporter(
		metrics.ExporterOptions{
			Domain:    metrics.Domain(),
			Component: component,
			ConfigMap: cfg,
		}, logger); err != nil {
		log.Fatalf("Failed to create the metrics exporter: %v", err)
	}

	// Register the views
	if err := view.Register(
		client.LatencyView,
		transporthttp.LatencyView,
		json.LatencyView,
		xml.LatencyView,
		datacodec.LatencyView,
	); err != nil {
		log.Fatalf("Failed to register views: %v", err)
	}

	view.SetReportingPeriod(2 * time.Second)
}
