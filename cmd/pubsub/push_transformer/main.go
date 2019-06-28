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

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/transformer"
)

func main() {
	flag.Parse()

	logger, _ := logging.NewLogger("", "INFO") // TODO: use logging config map.
	defer logger.Sync()

	ctx := logging.WithLogger(signals.NewContext(), logger)

	startable := &transformer.Transformer{}

	logger.Info("Starting Pub/Sub Transformer.", zap.Any("transformer", startable))
	if err := startable.Start(ctx); err != nil {
		logger.Fatal("failed to start transformer: ", zap.Error(err))
	}
}
