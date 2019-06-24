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

	"github.com/GoogleCloudPlatform/cloud-run-events/pkg/pubsub/operations"
	"github.com/kelseyhightower/envconfig"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()

	logger, _ := logging.NewLogger("", "INFO") // TODO: use logging config map.
	defer logger.Sync()

	ctx := logging.WithLogger(context.Background(), logger)

	ops := operations.SubscriptionOps{}
	if err := envconfig.Process("", &ops); err != nil {
		logger.Fatal("Failed to process env var", zap.Error(err))
	}
	if err := ops.CreateClient(ctx); err != nil {
		logger.Fatalw("Failed to create Pub/Sub Client.", zap.Error(err))
	}
	if err := ops.Run(ctx); err != nil {
		logger.Fatalw("Failed to run Subscription Operation.", zap.Error(err))
	}
}
